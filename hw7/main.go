package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
)

// =============================================
// DATA MODELS
// =============================================

type Item struct {
	ItemID   string  `json:"item_id"`
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type Order struct {
	OrderID    string    `json:"order_id"`
	CustomerID int       `json:"customer_id"`
	Status     string    `json:"status"`
	Items      []Item    `json:"items"`
	CreatedAt  time.Time `json:"created_at"`
}

// =============================================
// PAYMENT PROCESSOR SIMULATION
// =============================================
// WHY a buffered channel?
// The assignment says time.Sleep doesn't truly block a goroutine's OS thread.
// A buffered channel of size 1 ensures only ONE payment processes at a time.
// This simulates a real payment processor that can only handle 1 request
// at a time with a 3-second processing time.
// Max throughput = 1 order / 3 seconds = 0.33 orders/sec

var paymentSlot = make(chan struct{}, 1)

func simulatePayment(orderID string) {
	// Acquire the single payment slot (blocks if slot is taken)
	paymentSlot <- struct{}{}
	// Release the slot when done
	defer func() { <-paymentSlot }()

	log.Printf("[PAYMENT] Processing payment for order %s (3s delay)...", orderID)
	time.Sleep(3 * time.Second)
	log.Printf("[PAYMENT] Payment completed for order %s", orderID)
}

// =============================================
// COUNTERS (for the /stats endpoint)
// =============================================

var (
	syncSuccess  int64
	asyncSuccess int64
)

// =============================================
// HTTP HANDLERS
// =============================================

// GET /health — required by ALB health checks
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// GET /stats — see how many orders processed
func statsHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]int64{
		"sync_success":  atomic.LoadInt64(&syncSuccess),
		"async_success": atomic.LoadInt64(&asyncSuccess),
	})
}

// POST /orders/sync
// This is the SYNCHRONOUS endpoint.
// The customer's HTTP request BLOCKS for 3 seconds while payment processes.
// Under flash sale load, this causes timeouts and failures.
func syncOrderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the incoming request
	var req struct {
		CustomerID int    `json:"customer_id"`
		Items      []Item `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create the order
	order := Order{
		OrderID:    uuid.New().String(),
		CustomerID: req.CustomerID,
		Status:     "pending",
		Items:      req.Items,
		CreatedAt:  time.Now(),
	}

	// SYNCHRONOUS: Block here for 3 seconds
	order.Status = "processing"
	simulatePayment(order.OrderID)
	order.Status = "completed"

	atomic.AddInt64(&syncSuccess, 1)

	// Return 200 OK (customer waited 3+ seconds for this!)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(order)
}

// POST /orders/async
// This is the ASYNCHRONOUS endpoint.
// We publish the order to SNS and immediately return 202 Accepted.
// The customer gets a response in <100ms. Payment happens in background.
func asyncOrderHandler(snsClient *sns.Client, topicArn string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			CustomerID int    `json:"customer_id"`
			Items      []Item `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		order := Order{
			OrderID:    uuid.New().String(),
			CustomerID: req.CustomerID,
			Status:     "pending",
			Items:      req.Items,
			CreatedAt:  time.Now(),
		}

		// Convert order to JSON
		orderJSON, err := json.Marshal(order)
		if err != nil {
			http.Error(w, "Failed to marshal order", http.StatusInternalServerError)
			return
		}

		// Publish to SNS (this takes ~10-50ms, NOT 3 seconds)
		_, err = snsClient.Publish(context.TODO(), &sns.PublishInput{
			TopicArn: aws.String(topicArn),
			Message:  aws.String(string(orderJSON)),
		})
		if err != nil {
			log.Printf("[ERROR] Failed to publish to SNS: %v", err)
			http.Error(w, "Failed to queue order", http.StatusInternalServerError)
			return
		}

		atomic.AddInt64(&asyncSuccess, 1)

		// Return 202 Accepted IMMEDIATELY
		order.Status = "accepted"
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(order)
	}
}

// =============================================
// SQS WORKER (Background Order Processor)
// =============================================
// This continuously polls SQS for messages and processes them.
// Each worker goroutine processes one message at a time (3s each).
// More goroutines = more parallel processing.

func startSQSWorkers(sqsClient *sqs.Client, queueURL string, numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			log.Printf("[WORKER-%d] Started, polling SQS...", workerID)

			for {
				// Long poll: wait up to 20 seconds for messages
				result, err := sqsClient.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
					QueueUrl:            aws.String(queueURL),
					MaxNumberOfMessages: 10,
					WaitTimeSeconds:     20,
				})
				if err != nil {
					log.Printf("[WORKER-%d] Error receiving: %v", workerID, err)
					time.Sleep(5 * time.Second)
					continue
				}

				// Process each message
				for _, msg := range result.Messages {
					// SNS wraps the original message in an envelope
					var snsEnvelope struct {
						Message string `json:"Message"`
					}
					if err := json.Unmarshal([]byte(*msg.Body), &snsEnvelope); err != nil {
						log.Printf("[WORKER-%d] Error parsing SNS envelope: %v", workerID, err)
						continue
					}

					// Parse the actual order
					var order Order
					if err := json.Unmarshal([]byte(snsEnvelope.Message), &order); err != nil {
						log.Printf("[WORKER-%d] Error parsing order: %v", workerID, err)
						continue
					}

					log.Printf("[WORKER-%d] Processing order %s...", workerID, order.OrderID)

					// Simulate 3-second payment processing
					time.Sleep(3 * time.Second)

					// Delete message from queue (marks it as processed)
					_, delErr := sqsClient.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
						QueueUrl:      aws.String(queueURL),
						ReceiptHandle: msg.ReceiptHandle,
					})
					if delErr != nil {
						log.Printf("[WORKER-%d] Error deleting message: %v", workerID, delErr)
					} else {
						log.Printf("[WORKER-%d] Completed order %s", workerID, order.OrderID)
					}
				}
			}
		}(i)
	}
}

// =============================================
// MAIN — ENTRY POINT
// =============================================

func main() {
	// Read configuration from environment variables
	// These are set in the ECS task definition (Terraform)
	snsTopicArn := os.Getenv("SNS_TOPIC_ARN")
	sqsQueueURL := os.Getenv("SQS_QUEUE_URL")
	role := os.Getenv("SERVICE_ROLE") // "receiver" or "processor"
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Parse number of worker goroutines
	numWorkers := 1
	if n, err := strconv.Atoi(os.Getenv("NUM_WORKERS")); err == nil && n > 0 {
		numWorkers = n
	}

	// Initialize AWS SDK
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	snsClient := sns.NewFromConfig(cfg)
	sqsClient := sqs.NewFromConfig(cfg)

	// Register health check (both roles need this for ALB/ECS)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/stats", statsHandler)

	switch role {
	case "processor":
		// ── PROCESSOR ROLE ──
		// This ECS task only runs background workers polling SQS.
		// It does NOT handle customer HTTP requests.
		log.Printf("Starting ORDER PROCESSOR with %d workers", numWorkers)
		startSQSWorkers(sqsClient, sqsQueueURL, numWorkers)

	default:
		// ── RECEIVER ROLE ──
		// This ECS task handles incoming HTTP requests from customers.
		// It has both the /sync and /async endpoints.
		log.Println("Starting ORDER RECEIVER")
		http.HandleFunc("/orders/sync", syncOrderHandler)
		http.HandleFunc("/orders/async", asyncOrderHandler(snsClient, snsTopicArn))
	}

	log.Printf("Listening on port %s (role: %s)", port, role)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}