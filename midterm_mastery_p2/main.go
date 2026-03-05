package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ── Data model ────────────────────────────────────────────────────────────────

type Product struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Brand       string `json:"brand"`
}

type Recommendation struct {
	ProductID   int    `json:"product_id"`
	ProductName string `json:"product_name"`
	Reason      string `json:"reason"`
}

type SearchResponse struct {
	Products        []Product        `json:"products"`
	TotalFound      int              `json:"total_found"`
	SearchTime      string           `json:"search_time"`
	Recommendations []Recommendation `json:"recommendations"`
	RecoStatus      string           `json:"reco_status"` // tells client what happened
}

// ── Circuit Breaker ───────────────────────────────────────────────────────────

type CircuitState string

const (
	StateClosed   CircuitState = "CLOSED"    // normal - requests flow through
	StateOpen     CircuitState = "OPEN"      // tripped - requests blocked immediately
	StateHalfOpen CircuitState = "HALF_OPEN" // testing - a few requests allowed
)

type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitState
	failureCount     int
	successCount     int
	failureThreshold int           // how many failures before opening
	successThreshold int           // how many successes to close again
	timeout          time.Duration // how long to stay open before trying half-open
	lastFailureTime  time.Time
}

func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: 3,                // open after 3 failures
		successThreshold: 2,                // close after 2 successes in half-open
		timeout:          10 * time.Second, // stay open for 10 seconds
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true // normal operation

	case StateOpen:
		// check if timeout has passed - if so move to half-open
		if time.Since(cb.lastFailureTime) > cb.timeout {
			log.Printf("🔄 Circuit moving OPEN → HALF_OPEN, testing recovery...")
			cb.state = StateHalfOpen
			cb.successCount = 0
			return true
		}
		log.Printf("⛔ Circuit OPEN - blocking request to recommendations")
		return false // still open, block request

	case StateHalfOpen:
		return true // allow test requests through
	}
	return true
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			log.Printf("✅ Circuit moving HALF_OPEN → CLOSED, service recovered!")
			cb.state = StateClosed
			cb.failureCount = 0
		}
	}
	// NOTE: we do NOT reset failureCount on success in CLOSED state
	// This means failures accumulate even if there are occasional successes
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.state == StateHalfOpen {
		log.Printf("❌ Failure in HALF_OPEN - moving back to OPEN")
		cb.state = StateOpen
		return
	}

	if cb.state == StateClosed && cb.failureCount >= cb.failureThreshold {
		log.Printf("🔴 Circuit TRIPPED! Moving CLOSED → OPEN after %d failures", cb.failureCount)
		cb.state = StateOpen
	}
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// ── Seed data & products ──────────────────────────────────────────────────────

var (
	brands       = []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta", "Eta", "Theta", "Iota", "Kappa"}
	categories   = []string{"Electronics", "Books", "Home", "Sports", "Clothing", "Toys", "Garden", "Automotive", "Health", "Music"}
	descriptions = []string{
		"High quality product with excellent features.",
		"Affordable and reliable for everyday use.",
		"Premium grade, built to last.",
		"Lightweight design with superior performance.",
		"Customer favourite with top-rated reviews.",
	}
	products []Product
	cb       = NewCircuitBreaker()

	// Where the recommendations service lives
	recoServiceURL = "http://localhost:8081/recommendations"
)

func generateProducts() {
	products = make([]Product, 100_000)
	for i := 0; i < 100_000; i++ {
		brand := brands[i%len(brands)]
		products[i] = Product{
			ID:          i + 1,
			Name:        fmt.Sprintf("Product %s %d", brand, i+1),
			Category:    categories[i%len(categories)],
			Description: descriptions[i%len(descriptions)],
			Brand:       brand,
		}
	}
	log.Printf("Generated %d products", len(products))
}

// ── Get recommendations with circuit breaker + fail fast ─────────────────────

func getRecommendations() ([]Recommendation, string) {
	// CIRCUIT BREAKER CHECK - fail fast if circuit is open
	if !cb.Allow() {
		return nil, "circuit_open" // fail fast - don't even try
	}

	// FAIL FAST - short timeout so slow service doesn't hold up search
	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get(recoServiceURL)
	if err != nil {
		cb.RecordFailure()
		log.Printf("❌ Recommendations failed: %v", err)
		return nil, "unavailable"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cb.RecordFailure()
		log.Printf("❌ Recommendations returned %d", resp.StatusCode)
		return nil, "error"
	}

	var recs []Recommendation
	if err := json.NewDecoder(resp.Body).Decode(&recs); err != nil {
		cb.RecordFailure()
		return nil, "parse_error"
	}

	cb.RecordSuccess()
	return recs, "ok"
}

// ── Search handler ────────────────────────────────────────────────────────────

func searchHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if query == "" {
		http.Error(w, `{"error":"query parameter 'q' is required"}`, http.StatusBadRequest)
		return
	}

	// Search exactly 100 products
	var results []Product
	checked := 0
	for i := 0; i < len(products) && checked < 100; i++ {
		checked++
		p := products[i]
		if strings.Contains(strings.ToLower(p.Name), query) ||
			strings.Contains(strings.ToLower(p.Category), query) {
			results = append(results, p)
		}
	}

	totalFound := len(results)
	if len(results) > 20 {
		results = results[:20]
	}

	// BULKHEAD: recommendations run independently
	// Search ALWAYS returns results even if recommendations fails
	recs, recoStatus := getRecommendations()

	resp := SearchResponse{
		Products:        results,
		TotalFound:      totalFound,
		SearchTime:      fmt.Sprintf("%.3fs", time.Since(start).Seconds()),
		Recommendations: recs,
		RecoStatus:      recoStatus, // tells us what happened with recommendations
	}

	// Log circuit breaker state on every request
	log.Printf("Search done | circuit=%s | reco=%s | time=%s",
		cb.State(), recoStatus, resp.SearchTime)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ── Circuit breaker status endpoint ──────────────────────────────────────────

func circuitHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"circuit_state": string(cb.State()),
	})
}

// ── Health check ──────────────────────────────────────────────────────────────

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	log.Println("Starting product search service with circuit breaker...")
	generateProducts()

	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/circuit", circuitHandler)

	log.Println("Listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
