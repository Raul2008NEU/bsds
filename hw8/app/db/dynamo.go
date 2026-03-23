package db

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

var DynamoClient *dynamodb.Client
var DynamoTable string

// InitDynamo sets up the DynamoDB client.
// Returns an error if the table name env var is missing,
// but this is non-fatal — allows MySQL-only operation during Step I.
func InitDynamo() error {
	DynamoTable = os.Getenv("DYNAMODB_TABLE")
	if DynamoTable == "" {
		return fmt.Errorf("DYNAMODB_TABLE not set, skipping DynamoDB init")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(os.Getenv("AWS_REGION")),
	)
	if err != nil {
		return fmt.Errorf("aws config load failed: %w", err)
	}

	DynamoClient = dynamodb.NewFromConfig(cfg)
	log.Printf("DynamoDB client ready (table: %s)", DynamoTable)
	return nil
}