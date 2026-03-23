package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var MySQL *sql.DB

// InitMySQL opens a connection pool and runs schema migration.
//
// Connection pool settings:
//   - MaxOpenConns=25:  enough for 100 concurrent sessions with headroom
//   - MaxIdleConns=10:  keeps warm connections to avoid reconnect latency
//   - ConnMaxLifetime=5m: recycles connections to handle RDS failovers
func InitMySQL() error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	var err error
	MySQL, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("sql.Open failed: %w", err)
	}

	// Connection pool tuning
	MySQL.SetMaxOpenConns(25)
	MySQL.SetMaxIdleConns(10)
	MySQL.SetConnMaxLifetime(5 * time.Minute)

	// Verify connectivity (retry a few times — RDS can be slow on first connect)
	for i := 0; i < 10; i++ {
		err = MySQL.Ping()
		if err == nil {
			break
		}
		log.Printf("Waiting for MySQL (attempt %d/10): %v", i+1, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("mysql ping failed after retries: %w", err)
	}

	log.Println("Connected to MySQL")

	// Auto-migrate schema
	if err := migrate(); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}

// migrate creates tables if they don't exist.
//
// Schema design decisions:
//   - Two tables (carts + cart_items) for proper normalization
//   - UUID primary keys to avoid auto-increment contention across services
//   - Foreign key with CASCADE delete prevents orphaned items
//   - Index on customer_id for "get all carts by customer" queries
//   - Index on cart_id in items table is implicit via foreign key
//   - UNIQUE constraint on (cart_id, product_id) enables INSERT ... ON DUPLICATE KEY UPDATE
//     so adding the same product again just updates the quantity
func migrate() error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS carts (
			id VARCHAR(36) PRIMARY KEY,
			customer_id VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_customer_id (customer_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS cart_items (
			id INT AUTO_INCREMENT PRIMARY KEY,
			cart_id VARCHAR(36) NOT NULL,
			product_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			quantity INT NOT NULL DEFAULT 1,
			price DECIMAL(10,2) NOT NULL,
			FOREIGN KEY (cart_id) REFERENCES carts(id) ON DELETE CASCADE,
			UNIQUE KEY unique_cart_product (cart_id, product_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, s := range schema {
		if _, err := MySQL.Exec(s); err != nil {
			return fmt.Errorf("exec schema failed: %w\nSQL: %s", err, s)
		}
	}

	log.Println("MySQL schema migration complete")
	return nil
}