package main

import (
	"log"
	"net/http"
	"os"

	"hw8-store/db"
	"hw8-store/handlers"

	"github.com/gorilla/mux"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// --- Initialize MySQL ---
	if err := db.InitMySQL(); err != nil {
		log.Fatalf("Failed to init MySQL: %v", err)
	}
	log.Println("MySQL ready")

	// --- Initialize DynamoDB ---
	if err := db.InitDynamo(); err != nil {
		log.Printf("DynamoDB init skipped or failed: %v", err)
		// Don't fatal — allows running with MySQL only during Step I
	} else {
		log.Println("DynamoDB ready")
	}

	// --- Router ---
	r := mux.NewRouter()

	// Health check (ALB uses this)
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// MySQL endpoints (Step I)
	r.HandleFunc("/mysql/shopping-carts", handlers.MySQLCreateCart).Methods("POST")
	r.HandleFunc("/mysql/shopping-carts/{id}", handlers.MySQLGetCart).Methods("GET")
	r.HandleFunc("/mysql/shopping-carts/{id}/items", handlers.MySQLAddItem).Methods("POST")

	// DynamoDB endpoints (Step II)
	r.HandleFunc("/dynamo/shopping-carts", handlers.DynamoCreateCart).Methods("POST")
	r.HandleFunc("/dynamo/shopping-carts/{id}", handlers.DynamoGetCart).Methods("GET")
	r.HandleFunc("/dynamo/shopping-carts/{id}/items", handlers.DynamoAddItem).Methods("POST")

	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}