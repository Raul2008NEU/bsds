package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"hw8-store/db"
	"hw8-store/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// dynamoCart is the DynamoDB item structure.
// Items are embedded as a JSON list attribute inside the cart item.
// This means GetItem returns everything in one call — no Query needed.
type dynamoCart struct {
	CartID     string            `dynamodbav:"cart_id"`
	CustomerID string            `dynamodbav:"customer_id"`
	CreatedAt  string            `dynamodbav:"created_at"`
	UpdatedAt  string            `dynamodbav:"updated_at"`
	Items      []dynamoCartItem  `dynamodbav:"items"`
}

type dynamoCartItem struct {
	ProductID string  `dynamodbav:"product_id"`
	Name      string  `dynamodbav:"name"`
	Quantity  int     `dynamodbav:"quantity"`
	Price     float64 `dynamodbav:"price"`
}

// DynamoCreateCart handles POST /dynamo/shopping-carts
func DynamoCreateCart(w http.ResponseWriter, r *http.Request) {
	var req models.CreateCartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.CustomerID == "" {
		http.Error(w, `{"error":"customer_id is required"}`, http.StatusBadRequest)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	cart := dynamoCart{
		CartID:     uuid.New().String(),
		CustomerID: req.CustomerID,
		CreatedAt:  now,
		UpdatedAt:  now,
		Items:      []dynamoCartItem{},
	}

	item, err := attributevalue.MarshalMap(cart)
	if err != nil {
		log.Printf("DynamoDB marshal error: %v", err)
		http.Error(w, `{"error":"marshal failed"}`, http.StatusInternalServerError)
		return
	}

	_, err = db.DynamoClient.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(db.DynamoTable),
		Item:      item,
	})
	if err != nil {
		log.Printf("DynamoDB PutItem error: %v", err)
		http.Error(w, `{"error":"failed to create cart"}`, http.StatusInternalServerError)
		return
	}

	resp := models.Cart{
		ID:         cart.CartID,
		CustomerID: cart.CustomerID,
		Items:      []models.CartItem{},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// DynamoGetCart handles GET /dynamo/shopping-carts/{id}
// Single GetItem call — O(1) lookup by partition key.
func DynamoGetCart(w http.ResponseWriter, r *http.Request) {
	cartID := mux.Vars(r)["id"]

	result, err := db.DynamoClient.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String(db.DynamoTable),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: cartID},
		},
	})
	if err != nil {
		log.Printf("DynamoDB GetItem error: %v", err)
		http.Error(w, `{"error":"failed to get cart"}`, http.StatusInternalServerError)
		return
	}
	if result.Item == nil {
		http.Error(w, `{"error":"cart not found"}`, http.StatusNotFound)
		return
	}

	var cart dynamoCart
	if err := attributevalue.UnmarshalMap(result.Item, &cart); err != nil {
		log.Printf("DynamoDB unmarshal error: %v", err)
		http.Error(w, `{"error":"unmarshal failed"}`, http.StatusInternalServerError)
		return
	}

	// Convert to API response model
	createdAt, _ := time.Parse(time.RFC3339, cart.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, cart.UpdatedAt)

	resp := models.Cart{
		ID:         cart.CartID,
		CustomerID: cart.CustomerID,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		Items:      make([]models.CartItem, len(cart.Items)),
	}
	for i, item := range cart.Items {
		resp.Items[i] = models.CartItem{
			ProductID: item.ProductID,
			Name:      item.Name,
			Quantity:  item.Quantity,
			Price:     item.Price,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// DynamoAddItem handles POST /dynamo/shopping-carts/{id}/items
// Reads the cart, updates the items list in memory (upsert logic),
// then writes back. For a production system you'd use UpdateExpression,
// but this approach is clearer and sufficient for the assignment load.
func DynamoAddItem(w http.ResponseWriter, r *http.Request) {
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

	// Get current cart
	result, err := db.DynamoClient.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String(db.DynamoTable),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: cartID},
		},
	})
	if err != nil {
		log.Printf("DynamoDB GetItem error: %v", err)
		http.Error(w, `{"error":"failed to get cart"}`, http.StatusInternalServerError)
		return
	}
	if result.Item == nil {
		http.Error(w, `{"error":"cart not found"}`, http.StatusNotFound)
		return
	}

	var cart dynamoCart
	if err := attributevalue.UnmarshalMap(result.Item, &cart); err != nil {
		log.Printf("DynamoDB unmarshal error: %v", err)
		http.Error(w, `{"error":"unmarshal failed"}`, http.StatusInternalServerError)
		return
	}

	// Upsert: update existing product or append new one
	newItem := dynamoCartItem{
		ProductID: req.ProductID,
		Name:      req.Name,
		Quantity:  req.Quantity,
		Price:     req.Price,
	}

	found := false
	for i, item := range cart.Items {
		if item.ProductID == req.ProductID {
			cart.Items[i] = newItem
			found = true
			break
		}
	}
	if !found {
		cart.Items = append(cart.Items, newItem)
	}

	cart.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Write back
	item, err := attributevalue.MarshalMap(cart)
	if err != nil {
		log.Printf("DynamoDB marshal error: %v", err)
		http.Error(w, `{"error":"marshal failed"}`, http.StatusInternalServerError)
		return
	}

	_, err = db.DynamoClient.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(db.DynamoTable),
		Item:      item,
	})
	if err != nil {
		log.Printf("DynamoDB PutItem error: %v", err)
		http.Error(w, `{"error":"failed to update cart"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "item added", "cart_id": cartID})
}