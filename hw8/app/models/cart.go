package models

import "time"

// Cart represents a shopping cart
type Cart struct {
	ID         string     `json:"id"`
	CustomerID string     `json:"customer_id"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Items      []CartItem `json:"items"`
}

// CartItem represents a single item in a cart
type CartItem struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

// CreateCartRequest is the body for POST /shopping-carts
type CreateCartRequest struct {
	CustomerID string `json:"customer_id"`
}

// AddItemRequest is the body for POST /shopping-carts/{id}/items
type AddItemRequest struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}