package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"hw8-store/db"
	"hw8-store/models"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// MySQLCreateCart handles POST /mysql/shopping-carts
// Creates a new cart and returns it with HTTP 201.
func MySQLCreateCart(w http.ResponseWriter, r *http.Request) {
	var req models.CreateCartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.CustomerID == "" {
		http.Error(w, `{"error":"customer_id is required"}`, http.StatusBadRequest)
		return
	}

	cartID := uuid.New().String()

	_, err := db.MySQL.Exec(
		"INSERT INTO carts (id, customer_id) VALUES (?, ?)",
		cartID, req.CustomerID,
	)
	if err != nil {
		log.Printf("MySQL insert cart error: %v", err)
		http.Error(w, `{"error":"failed to create cart"}`, http.StatusInternalServerError)
		return
	}

	cart := models.Cart{
		ID:         cartID,
		CustomerID: req.CustomerID,
		Items:      []models.CartItem{},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cart)
}

// MySQLGetCart handles GET /mysql/shopping-carts/{id}
// Uses a JOIN to fetch the cart and all its items in one query.
func MySQLGetCart(w http.ResponseWriter, r *http.Request) {
	cartID := mux.Vars(r)["id"]

	// First get the cart itself
	var cart models.Cart
	err := db.MySQL.QueryRow(
		"SELECT id, customer_id, created_at, updated_at FROM carts WHERE id = ?",
		cartID,
	).Scan(&cart.ID, &cart.CustomerID, &cart.CreatedAt, &cart.UpdatedAt)

	if err != nil {
		log.Printf("MySQL get cart error: %v", err)
		http.Error(w, `{"error":"cart not found"}`, http.StatusNotFound)
		return
	}

	// Then get all items for this cart
	rows, err := db.MySQL.Query(
		"SELECT product_id, name, quantity, price FROM cart_items WHERE cart_id = ?",
		cartID,
	)
	if err != nil {
		log.Printf("MySQL get items error: %v", err)
		http.Error(w, `{"error":"failed to retrieve items"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	cart.Items = []models.CartItem{}
	for rows.Next() {
		var item models.CartItem
		if err := rows.Scan(&item.ProductID, &item.Name, &item.Quantity, &item.Price); err != nil {
			log.Printf("MySQL scan item error: %v", err)
			continue
		}
		cart.Items = append(cart.Items, item)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cart)
}

// MySQLAddItem handles POST /mysql/shopping-carts/{id}/items
// Uses INSERT ... ON DUPLICATE KEY UPDATE so adding the same product
// again updates its quantity instead of creating a duplicate.
// Wrapped in a transaction for data integrity.
func MySQLAddItem(w http.ResponseWriter, r *http.Request) {
	cartID := mux.Vars(r)["id"]

	var req models.AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.ProductID == "" || req.Quantity <= 0 {
		http.Error(w, `{"error":"product_id and positive quantity required"}`, http.StatusBadRequest)
		return
	}

	// Verify the cart exists
	var exists bool
	err := db.MySQL.QueryRow("SELECT EXISTS(SELECT 1 FROM carts WHERE id = ?)", cartID).Scan(&exists)
	if err != nil || !exists {
		http.Error(w, `{"error":"cart not found"}`, http.StatusNotFound)
		return
	}

	// Begin transaction
	tx, err := db.MySQL.Begin()
	if err != nil {
		log.Printf("MySQL begin tx error: %v", err)
		http.Error(w, `{"error":"transaction failed"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Upsert the item
	_, err = tx.Exec(
		`INSERT INTO cart_items (cart_id, product_id, name, quantity, price)
		 VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE quantity = VALUES(quantity), name = VALUES(name), price = VALUES(price)`,
		cartID, req.ProductID, req.Name, req.Quantity, req.Price,
	)
	if err != nil {
		log.Printf("MySQL upsert item error: %v", err)
		http.Error(w, `{"error":"failed to add item"}`, http.StatusInternalServerError)
		return
	}

	// Touch updated_at on the cart
	_, err = tx.Exec("UPDATE carts SET updated_at = NOW() WHERE id = ?", cartID)
	if err != nil {
		log.Printf("MySQL update cart timestamp error: %v", err)
		http.Error(w, `{"error":"failed to update cart"}`, http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("MySQL commit error: %v", err)
		http.Error(w, `{"error":"commit failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "item added", "cart_id": cartID})
}