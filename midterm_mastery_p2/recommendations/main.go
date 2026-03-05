package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"time"
)

// This service simulates a real-world flaky dependency.
// It randomly:
//   - Responds normally (30% of the time)
//   - Responds slowly - 5 second delay (50% of the time)
//   - Crashes with 500 error (20% of the time)

type Recommendation struct {
	ProductID   int    `json:"product_id"`
	ProductName string `json:"product_name"`
	Reason      string `json:"reason"`
}

func recommendHandler(w http.ResponseWriter, r *http.Request) {
	roll := rand.Float32()

	if roll < 0.50 {
		// 50% chance: crash with 500
		log.Printf("💥 Simulating crash - returning 500")
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	if roll < 0.80 {
		// 30% chance: slow response (5 seconds)
		log.Printf("🐌 Simulating slow response - sleeping 5 seconds")
		time.Sleep(5 * time.Second)
	}

	// 30% chance (or after slow): normal response
	recs := []Recommendation{
		{ProductID: 1, ProductName: "Product Alpha 1", Reason: "Popular in your category"},
		{ProductID: 2, ProductName: "Product Beta 2", Reason: "Customers also bought"},
		{ProductID: 3, ProductName: "Product Gamma 3", Reason: "Trending this week"},
	}

	log.Printf("✅ Returning recommendations normally")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recs)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func main() {
	log.Println("Starting broken recommendations service on :8081")
	http.HandleFunc("/recommendations", recommendHandler)
	http.HandleFunc("/health", healthHandler)
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
