package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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

type SearchResponse struct {
	Products   []Product `json:"products"`
	TotalFound int       `json:"total_found"`
	SearchTime string    `json:"search_time"`
}

// ── Seed data ─────────────────────────────────────────────────────────────────

var brands = []string{
	"Alpha", "Beta", "Gamma", "Delta", "Epsilon",
	"Zeta", "Eta", "Theta", "Iota", "Kappa",
}

var categories = []string{
	"Electronics", "Books", "Home", "Sports",
	"Clothing", "Toys", "Garden", "Automotive",
	"Health", "Music",
}

var descriptions = []string{
	"High quality product with excellent features.",
	"Affordable and reliable for everyday use.",
	"Premium grade, built to last.",
	"Lightweight design with superior performance.",
	"Customer favourite with top-rated reviews.",
}

// ── In-memory product store ───────────────────────────────────────────────────

var products []Product // 100,000 items loaded at startup

func generateProducts() {
	products = make([]Product, 100_000)
	for i := 0; i < 100_000; i++ {
		brand := brands[i%len(brands)]
		category := categories[i%len(categories)]
		products[i] = Product{
			ID:          i + 1,
			Name:        fmt.Sprintf("Product %s %d", brand, i+1),
			Category:    category,
			Description: descriptions[i%len(descriptions)],
			Brand:       brand,
		}
	}
	log.Printf("Generated %d products\n", len(products))
}

// ── Search handler ────────────────────────────────────────────────────────────

func searchHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if query == "" {
		http.Error(w, `{"error":"query parameter 'q' is required"}`, http.StatusBadRequest)
		return
	}

	var results []Product
	checked := 0

	// CRITICAL: check exactly 100 products, then stop.
	for i := 0; i < len(products) && checked < 100; i++ {
		checked++ // increment for EVERY product looked at, not just matches
		p := products[i]
		if strings.Contains(strings.ToLower(p.Name), query) ||
			strings.Contains(strings.ToLower(p.Category), query) {
			results = append(results, p)
		}
	}

	totalFound := len(results)

	// Cap returned results at 20
	if len(results) > 20 {
		results = results[:20]
	}

	resp := SearchResponse{
		Products:   results,
		TotalFound: totalFound,
		SearchTime: fmt.Sprintf("%.3fs", time.Since(start).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ── Health check ──────────────────────────────────────────────────────────────

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	log.Println("Starting product search service...")
	generateProducts()

	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/health", healthHandler)

	port := "8080"
	log.Printf("Listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}