
package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

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

func handler(ctx context.Context, snsEvent events.SNSEvent) error {
	for _, record := range snsEvent.Records {
		// Parse the order from the SNS message
		var order Order
		if err := json.Unmarshal([]byte(record.SNS.Message), &order); err != nil {
			log.Printf("[LAMBDA] Error parsing order: %v", err)
			continue
		}

		log.Printf("[LAMBDA] Processing order %s for customer %d",
			order.OrderID, order.CustomerID)

		// Same 3-second payment simulation as ECS
		time.Sleep(3 * time.Second)

		log.Printf("[LAMBDA] Completed order %s", order.OrderID)
	}
	return nil
}

func main() {
	lambda.Start(handler)
}