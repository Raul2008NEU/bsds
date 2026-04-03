package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

type record struct {
	ts        time.Time
	rType     string // "read" | "write"
	key       string
	latencyMs float64
	version   int64
	stale     bool
}

// versionTracker keeps the highest version written per key (client-side).
type versionTracker struct {
	mu       sync.RWMutex
	versions map[string]int64
}

func (t *versionTracker) update(key string, ver int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if ver > t.versions[key] {
		t.versions[key] = ver
	}
}

func (t *versionTracker) latest(key string) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.versions[key]
}

// ─────────────────────────────────────────────────────────────────────────────
// Flags
// ─────────────────────────────────────────────────────────────────────────────

var (
	targetFlag   = flag.String("target", "", "comma-separated read-target addresses (e.g. localhost:8082,localhost:8083)")
	leaderFlag   = flag.String("leader", "", "leader address for writes in leader-follower mode (e.g. localhost:8081); if empty writes go to random --target")
	ratioFlag    = flag.Int("ratio", 10, "write percentage 0-100 (e.g. 10 = 10% writes, 90% reads)")
	durationFlag = flag.Duration("duration", 30*time.Second, "test duration (e.g. 30s)")
	keysFlag     = flag.Int("keys", 10, "number of unique keys")
	workersFlag  = flag.Int("workers", 10, "number of concurrent goroutines")
)

// ─────────────────────────────────────────────────────────────────────────────
// main
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	flag.Parse()

	if *targetFlag == "" {
		fmt.Fprintln(os.Stderr, "error: --target is required")
		flag.Usage()
		os.Exit(1)
	}

	targets := splitTrim(*targetFlag)

	keys := make([]string, *keysFlag)
	for i := range keys {
		keys[i] = fmt.Sprintf("key%d", i)
	}

	tracker := &versionTracker{versions: make(map[string]int64)}

	// Shared result buffer; workers append under resultsMu.
	var (
		resultsMu sync.Mutex
		results   []record
	)

	deadline := time.Now().Add(*durationFlag)

	var wg sync.WaitGroup
	for i := 0; i < *workersFlag; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each goroutine gets its own RNG to avoid lock contention.
			rng := rand.New(rand.NewSource(time.Now().UnixNano() ^ int64(rand.Int())))
			client := &http.Client{Timeout: 10 * time.Second}

			for time.Now().Before(deadline) {
				key := keys[rng.Intn(len(keys))]
				isWrite := rng.Intn(100) < *ratioFlag

				var rec record
				if isWrite {
					writeTarget := *leaderFlag
					if writeTarget == "" {
						writeTarget = targets[rng.Intn(len(targets))]
					}
					rec = doWrite(client, writeTarget, key, tracker)
				} else {
					rec = doRead(client, targets[rng.Intn(len(targets))], key, tracker)
				}

				resultsMu.Lock()
				results = append(results, rec)
				resultsMu.Unlock()
			}
		}()
	}

	wg.Wait()

	writeCSV(results)
	printSummary(results)
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP helpers
// ─────────────────────────────────────────────────────────────────────────────

func doWrite(client *http.Client, target, key string, tracker *versionTracker) record {
	value := fmt.Sprintf("v-%d", time.Now().UnixNano())
	body, _ := json.Marshal(map[string]string{"key": key, "value": value})

	start := time.Now()
	resp, err := client.Post("http://"+target+"/set", "application/json", bytes.NewReader(body))
	latency := float64(time.Since(start).Microseconds()) / 1000.0

	if err != nil {
		return record{ts: start, rType: "write", key: key, latencyMs: latency, version: -1}
	}
	defer resp.Body.Close()

	var respBody struct {
		Version int64 `json:"version"`
	}
	json.NewDecoder(resp.Body).Decode(&respBody)

	if respBody.Version > 0 {
		tracker.update(key, respBody.Version)
	}

	return record{ts: start, rType: "write", key: key, latencyMs: latency, version: respBody.Version}
}

func doRead(client *http.Client, target, key string, tracker *versionTracker) record {
	start := time.Now()
	resp, err := client.Get("http://" + target + "/get/" + key)
	latency := float64(time.Since(start).Microseconds()) / 1000.0

	if err != nil {
		return record{ts: start, rType: "read", key: key, latencyMs: latency, version: -1}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return record{ts: start, rType: "read", key: key, latencyMs: latency, version: 0}
	}

	var entry struct {
		Version int64 `json:"version"`
	}
	json.NewDecoder(resp.Body).Decode(&entry)

	expected := tracker.latest(key)
	stale := expected > 0 && entry.Version < expected

	return record{ts: start, rType: "read", key: key, latencyMs: latency, version: entry.Version, stale: stale}
}

// ─────────────────────────────────────────────────────────────────────────────
// Output
// ─────────────────────────────────────────────────────────────────────────────

func writeCSV(results []record) {
	w := csv.NewWriter(os.Stdout)
	_ = w.Write([]string{"timestamp", "type", "key", "latency_ms", "version", "stale"})
	for _, r := range results {
		_ = w.Write([]string{
			r.ts.Format(time.RFC3339Nano),
			r.rType,
			r.key,
			fmt.Sprintf("%.3f", r.latencyMs),
			strconv.FormatInt(r.version, 10),
			strconv.FormatBool(r.stale),
		})
	}
	w.Flush()
}

func printSummary(results []record) {
	var (
		totalReads, totalWrites, staleReads int
		readLat, writeLat                   []float64
	)
	for _, r := range results {
		switch r.rType {
		case "read":
			totalReads++
			if r.latencyMs >= 0 {
				readLat = append(readLat, r.latencyMs)
			}
			if r.stale {
				staleReads++
			}
		case "write":
			totalWrites++
			if r.latencyMs >= 0 {
				writeLat = append(writeLat, r.latencyMs)
			}
		}
	}

	fmt.Fprintf(os.Stderr, "\n========== Load Test Summary ==========\n")
	fmt.Fprintf(os.Stderr, "Total requests : %d\n", totalReads+totalWrites)
	fmt.Fprintf(os.Stderr, "  Reads        : %d\n", totalReads)
	fmt.Fprintf(os.Stderr, "  Writes       : %d\n", totalWrites)
	fmt.Fprintf(os.Stderr, "Stale reads    : %d / %d  (%.2f%%)\n",
		staleReads, totalReads, pct(staleReads, totalReads))
	fmt.Fprintf(os.Stderr, "\nRead  latency  : avg=%6.2fms  p50=%6.2fms  p95=%6.2fms  p99=%6.2fms\n",
		avg(readLat), percentile(readLat, 50), percentile(readLat, 95), percentile(readLat, 99))
	fmt.Fprintf(os.Stderr, "Write latency  : avg=%6.2fms  p50=%6.2fms  p95=%6.2fms  p99=%6.2fms\n",
		avg(writeLat), percentile(writeLat, 50), percentile(writeLat, 95), percentile(writeLat, 99))
	fmt.Fprintf(os.Stderr, "=======================================\n")
}

// ─────────────────────────────────────────────────────────────────────────────
// Stats helpers
// ─────────────────────────────────────────────────────────────────────────────

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func percentile(vals []float64, p float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	idx := int(float64(len(sorted)-1) * p / 100.0)
	return sorted[idx]
}

func splitTrim(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
