package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// KVEntry holds a single key-value pair with a logical version counter.
type KVEntry struct {
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

// SetRequest is the JSON body for POST /set and POST /internal/set.
type SetRequest struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int64  `json:"version,omitempty"`
}

// GetResponse is the JSON body returned on GET /get/:key and GET /local_read/:key.
type GetResponse struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

var (
	nodeID  string
	role    string
	peers   []string
	rQuorum int
	wQuorum int

	mu    sync.RWMutex
	store = make(map[string]KVEntry)

	httpClient = &http.Client{Timeout: 15 * time.Second}
)

func logf(format string, args ...interface{}) {
	log.Printf("["+nodeID+"] "+format, args...)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ─────────────────────────────────────────────────────────────────────────────
// main
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	nodeID = getEnv("NODE_ID", "node1")
	role = strings.ToLower(getEnv("ROLE", "leader"))
	port := getEnv("PORT", "8080")

	if p := getEnv("PEERS", ""); p != "" {
		for _, peer := range strings.Split(p, ",") {
			peer = strings.TrimSpace(peer)
			if peer != "" {
				peers = append(peers, peer)
			}
		}
	}

	r, _ := strconv.Atoi(getEnv("R", "1"))
	w, _ := strconv.Atoi(getEnv("W", "5"))
	rQuorum = r
	wQuorum = w

	logf("starting: role=%s peers=%v R=%d W=%d port=%s", role, peers, rQuorum, wQuorum, port)

	mux := http.NewServeMux()
	mux.HandleFunc("/set", handleSet)
	mux.HandleFunc("/get/", handleGet)
	mux.HandleFunc("/local_read/", handleLocalRead)
	mux.HandleFunc("/internal/set", handleInternalSet)
	mux.HandleFunc("/internal/get/", handleInternalGet)

	log.Fatal(http.ListenAndServe(":"+port, mux))
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /set
// ─────────────────────────────────────────────────────────────────────────────

func handleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	var req SetRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Key == "" {
		http.Error(w, "key cannot be empty", http.StatusBadRequest)
		return
	}

	mu.Lock()
	entry := store[req.Key]
	newVersion := entry.Version + 1
	store[req.Key] = KVEntry{Value: req.Value, Version: newVersion}
	mu.Unlock()

	logf("SET key=%s value=%s version=%d", req.Key, req.Value, newVersion)

	replicateWrite(req.Key, req.Value, newVersion)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"key":     req.Key,
		"value":   req.Value,
		"version": newVersion,
	})
}

// replicateWrite sends /internal/set to peers.
//   - W=1 → fully async.
//   - W=3 → wait for 2 peer acks, rest async.
//   - W=5 → wait for all peer acks.
//
// 200 ms sleep between each sequential send (spec requirement).
func replicateWrite(key, value string, version int64) {
	if len(peers) == 0 {
		return
	}

	syncNeeded := wQuorum - 1 // local write already counts as 1
	if syncNeeded < 0 {
		syncNeeded = 0
	}
	if syncNeeded > len(peers) {
		syncNeeded = len(peers)
	}

	if wQuorum == 1 {
		go func() {
			for _, peer := range peers {
				if err := sendInternalSet(peer, key, value, version); err != nil {
					logf("async replication error to %s: %v", peer, err)
				}
				time.Sleep(200 * time.Millisecond)
			}
		}()
		return
	}

	for i := 0; i < syncNeeded; i++ {
		if err := sendInternalSet(peers[i], key, value, version); err != nil {
			logf("replication error to %s: %v", peers[i], err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	if syncNeeded < len(peers) {
		remaining := make([]string, len(peers)-syncNeeded)
		copy(remaining, peers[syncNeeded:])
		go func() {
			for _, peer := range remaining {
				if err := sendInternalSet(peer, key, value, version); err != nil {
					logf("async replication error to %s: %v", peer, err)
				}
				time.Sleep(200 * time.Millisecond)
			}
		}()
	}
}

func sendInternalSet(peer, key, value string, version int64) error {
	body, _ := json.Marshal(SetRequest{Key: key, Value: value, Version: version})
	resp, err := httpClient.Post("http://"+peer+"/internal/set", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("peer %s returned HTTP %d", peer, resp.StatusCode)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /get/:key
// ─────────────────────────────────────────────────────────────────────────────

func handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/get/")
	if key == "" {
		http.Error(w, "key cannot be empty", http.StatusBadRequest)
		return
	}

	logf("GET key=%s", key)

	if role == "leader" && rQuorum > 1 {
		fanOutRead(w, key)
		return
	}

	localRespond(w, key)
}

// fanOutRead reads from local + (R-1) peers concurrently, returns highest version.
func fanOutRead(w http.ResponseWriter, key string) {
	fanCount := rQuorum - 1
	if fanCount > len(peers) {
		fanCount = len(peers)
	}

	type result struct {
		entry  KVEntry
		exists bool
	}

	ch := make(chan result, fanCount+1)

	mu.RLock()
	localEntry, localExists := store[key]
	mu.RUnlock()
	ch <- result{localEntry, localExists}

	for i := 0; i < fanCount; i++ {
		go func(peer string) {
			resp, err := httpClient.Get("http://" + peer + "/internal/get/" + key)
			if err != nil {
				logf("fan-out read error from %s: %v", peer, err)
				ch <- result{exists: false}
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusNotFound {
				ch <- result{exists: false}
				return
			}
			var entry KVEntry
			if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
				logf("fan-out decode error from %s: %v", peer, err)
				ch <- result{exists: false}
				return
			}
			ch <- result{entry, true}
		}(peers[i])
	}

	var best KVEntry
	found := false
	for i := 0; i < fanCount+1; i++ {
		res := <-ch
		if res.exists && (!found || res.entry.Version > best.Version) {
			best = res.entry
			found = true
		}
	}

	if !found {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetResponse{Key: key, Value: best.Value, Version: best.Version})
}

func localRespond(w http.ResponseWriter, key string) {
	mu.RLock()
	entry, exists := store[key]
	mu.RUnlock()

	if !exists {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetResponse{Key: key, Value: entry.Value, Version: entry.Version})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /local_read/:key
// ─────────────────────────────────────────────────────────────────────────────

func handleLocalRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/local_read/")
	if key == "" {
		http.Error(w, "key cannot be empty", http.StatusBadRequest)
		return
	}

	localRespond(w, key)
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /internal/set
// ─────────────────────────────────────────────────────────────────────────────

func handleInternalSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	time.Sleep(100 * time.Millisecond) // spec: follower sleeps 100 ms on write

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}
	var req SetRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Key == "" {
		http.Error(w, "key cannot be empty", http.StatusBadRequest)
		return
	}

	mu.Lock()
	store[req.Key] = KVEntry{Value: req.Value, Version: req.Version}
	mu.Unlock()

	logf("INTERNAL_SET key=%s version=%d", req.Key, req.Version)
	w.WriteHeader(http.StatusOK)
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /internal/get/:key
// ─────────────────────────────────────────────────────────────────────────────

func handleInternalGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	time.Sleep(50 * time.Millisecond) // spec: follower sleeps 50 ms on read fan-out

	key := strings.TrimPrefix(r.URL.Path, "/internal/get/")
	if key == "" {
		http.Error(w, "key cannot be empty", http.StatusBadRequest)
		return
	}

	mu.RLock()
	entry, exists := store[key]
	mu.RUnlock()

	if !exists {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}
