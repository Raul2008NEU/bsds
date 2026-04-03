// Package tests contains black-box consistency tests for the distributed KV store.
// The Docker Compose cluster must already be running before executing these tests.
//
// Leader-follower cluster:  docker compose -f ../docker-compose.leader.yml up -d
// Leaderless cluster:       docker compose -f ../docker-compose.leaderless.yml up -d
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Addresses
// ─────────────────────────────────────────────────────────────────────────────

const (
	// Leader-follower cluster
	leaderAddr    = "http://localhost:8081"
	follower1Addr = "http://localhost:8082"
	follower2Addr = "http://localhost:8083"
	follower3Addr = "http://localhost:8084"
	follower4Addr = "http://localhost:8085"

	// Leaderless cluster (same ports, different compose file)
	node1Addr = "http://localhost:8081"
	node2Addr = "http://localhost:8082"
	node3Addr = "http://localhost:8083"
	node4Addr = "http://localhost:8084"
	node5Addr = "http://localhost:8085"
)

var followerAddrs = []string{follower1Addr, follower2Addr, follower3Addr, follower4Addr}
var allNodeAddrs = []string{node1Addr, node2Addr, node3Addr, node4Addr, node5Addr}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP helpers
// ─────────────────────────────────────────────────────────────────────────────

type kvResponse struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// writeKey sends POST /set to addr and returns the version from the response.
func writeKey(t *testing.T, addr, key, value string) (int64, error) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"key": key, "value": value})
	resp, err := httpClient.Post(addr+"/set", "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("POST /set to %s: %w", addr, err)
	}
	defer resp.Body.Close()
	var r kvResponse
	json.NewDecoder(resp.Body).Decode(&r)
	return r.Version, nil
}

// readKey sends GET /get/:key and returns nil if 404.
func readKey(t *testing.T, addr, key string) (*kvResponse, error) {
	t.Helper()
	resp, err := httpClient.Get(addr + "/get/" + key)
	if err != nil {
		return nil, fmt.Errorf("GET /get/%s from %s: %w", key, addr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	var r kvResponse
	json.NewDecoder(resp.Body).Decode(&r)
	return &r, nil
}

// localReadKey sends GET /local_read/:key and returns nil if 404.
func localReadKey(t *testing.T, addr, key string) (*kvResponse, error) {
	t.Helper()
	resp, err := httpClient.Get(addr + "/local_read/" + key)
	if err != nil {
		return nil, fmt.Errorf("GET /local_read/%s from %s: %w", key, addr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	var r kvResponse
	json.NewDecoder(resp.Body).Decode(&r)
	return &r, nil
}

// uniqueKey generates a test-scoped unique key to avoid cross-test pollution.
func uniqueKey(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// ─────────────────────────────────────────────────────────────────────────────
// Leader-Follower tests
// ─────────────────────────────────────────────────────────────────────────────

// TestLeaderWriteLeaderRead writes to the leader and immediately reads back from
// the leader. Expects the same version — no replication involved.
func TestLeaderWriteLeaderRead(t *testing.T) {
	key := uniqueKey("llr")
	value := "leader-value"

	ver, err := writeKey(t, leaderAddr, key, value)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	got, err := readKey(t, leaderAddr, key)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if got == nil {
		t.Fatal("key not found on leader after write")
	}
	if got.Value != value {
		t.Errorf("value mismatch: want %q got %q", value, got.Value)
	}
	if got.Version != ver {
		t.Errorf("version mismatch: want %d got %d", ver, got.Version)
	}
}

// TestLeaderWriteFollowerRead writes to the leader with W=5 (cluster default).
// After the write ack all followers must have the data — reads every follower.
func TestLeaderWriteFollowerRead(t *testing.T) {
	key := uniqueKey("lfr")
	value := "replicated-value"

	ver, err := writeKey(t, leaderAddr, key, value)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	for _, f := range followerAddrs {
		f := f
		t.Run(f, func(t *testing.T) {
			got, err := readKey(t, f, key)
			if err != nil {
				t.Errorf("read from %s failed: %v", f, err)
				return
			}
			if got == nil {
				t.Errorf("key not found on follower %s after W=5 ack", f)
				return
			}
			if got.Value != value {
				t.Errorf("%s value mismatch: want %q got %q", f, value, got.Value)
			}
			if got.Version != ver {
				t.Errorf("%s version mismatch: want %d got %d", f, ver, got.Version)
			}
		})
	}
}

// TestLocalReadInconsistency writes to the leader and immediately fires
// local_read on all followers in parallel — before replication completes.
// Inconsistencies (key missing or stale version) are expected and logged.
// The test runs 50 iterations to reliably catch the race.
func TestLocalReadInconsistency(t *testing.T) {
	const iterations = 50
	var inconsistencies int

	for i := 0; i < iterations; i++ {
		key := uniqueKey(fmt.Sprintf("lri-%d", i))
		value := fmt.Sprintf("val-%d", i)

		// Fire the write asynchronously so we can overlap with reads.
		writeDone := make(chan int64, 1)
		go func() {
			ver, _ := writeKey(t, leaderAddr, key, value)
			writeDone <- ver
		}()

		// Immediately local_read from all followers in parallel.
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, f := range followerAddrs {
			wg.Add(1)
			go func(addr string) {
				defer wg.Done()
				got, _ := localReadKey(t, addr, key)
				if got == nil {
					mu.Lock()
					inconsistencies++
					t.Logf("[iter %d] inconsistency: key %s not yet on %s", i, key, addr)
					mu.Unlock()
				}
			}(f)
		}
		wg.Wait()
		<-writeDone // drain channel; don't leak goroutines
	}

	t.Logf("Total inconsistencies: %d / %d local_reads", inconsistencies, iterations*len(followerAddrs))
	// We don't fail — inconsistency is the expected behaviour being demonstrated.
}

// ─────────────────────────────────────────────────────────────────────────────
// Leaderless tests
// ─────────────────────────────────────────────────────────────────────────────

// TestLeaderlessCoordinatorConsistency writes to node1 (coordinator) and reads
// back from node1 after the ack. With W=5 the coordinator itself already has
// the data, so this must always be consistent.
func TestLeaderlessCoordinatorConsistency(t *testing.T) {
	key := uniqueKey("lcc")
	value := "coordinator-value"

	ver, err := writeKey(t, node1Addr, key, value)
	if err != nil {
		t.Fatalf("write to node1 failed: %v", err)
	}

	got, err := readKey(t, node1Addr, key)
	if err != nil {
		t.Fatalf("read from node1 failed: %v", err)
	}
	if got == nil {
		t.Fatal("key not found on node1 after write")
	}
	if got.Value != value {
		t.Errorf("value mismatch: want %q got %q", value, got.Value)
	}
	if got.Version != ver {
		t.Errorf("version mismatch: want %d got %d", ver, got.Version)
	}
}

// TestLeaderlessEventualConsistency writes to node1 with W=5 (all nodes must
// ack before the coordinator responds). After the ack, node3 must have the
// data since it is one of the peers that already acknowledged.
func TestLeaderlessEventualConsistency(t *testing.T) {
	key := uniqueKey("lec")
	value := "eventual-value"

	ver, err := writeKey(t, node1Addr, key, value)
	if err != nil {
		t.Fatalf("write to node1 failed: %v", err)
	}

	// After W=5 ack every node must be consistent.
	for _, n := range allNodeAddrs {
		n := n
		t.Run(n, func(t *testing.T) {
			got, err := readKey(t, n, key)
			if err != nil {
				t.Errorf("read from %s failed: %v", n, err)
				return
			}
			if got == nil {
				t.Errorf("key not found on %s after W=5 ack", n)
				return
			}
			if got.Value != value {
				t.Errorf("%s value mismatch: want %q got %q", n, value, got.Value)
			}
			if got.Version != ver {
				t.Errorf("%s version mismatch: want %d got %d", n, ver, got.Version)
			}
		})
	}
}

// TestLeaderlessInconsistencyWindow writes to node1 and immediately fires
// local_read on the other nodes in parallel — catching the replication window
// before the coordinator has finished sending to all peers.
// Inconsistencies are logged (not fatal) as they are the intended observation.
func TestLeaderlessInconsistencyWindow(t *testing.T) {
	const iterations = 50
	var inconsistencies int

	peers := allNodeAddrs[1:] // node2-5; node1 is coordinator and always consistent

	for i := 0; i < iterations; i++ {
		key := uniqueKey(fmt.Sprintf("liw-%d", i))
		value := fmt.Sprintf("val-%d", i)

		writeDone := make(chan struct{}, 1)
		go func() {
			writeKey(t, node1Addr, key, value) //nolint:errcheck
			writeDone <- struct{}{}
		}()

		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, n := range peers {
			wg.Add(1)
			go func(addr string) {
				defer wg.Done()
				got, _ := localReadKey(t, addr, key)
				if got == nil {
					mu.Lock()
					inconsistencies++
					t.Logf("[iter %d] inconsistency: key %s not yet visible on %s", i, key, addr)
					mu.Unlock()
				}
			}(n)
		}
		wg.Wait()
		<-writeDone
	}

	t.Logf("Total inconsistencies: %d / %d local_reads", inconsistencies, iterations*len(peers))
}
