package database

import (
	"database/sql"
	"fmt"
	"tradingbot/internal/models"

	_ "github.com/go-sql-driver/mysql"
)

type DB struct {
	*sql.DB
}

// NewConnection establishes a new connection to the database and returns a DB instance.
// It verifies the connection by pinging the database.
func NewConnection(databaseURL string) (*DB, error) {
	db, err := sql.Open("mysql", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %v", err)
	}

	// Verify the connection is valid by pinging the database
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	return &DB{db}, nil
}

// SaveOrder saves a new order record to the database.
// Returns an error if the insertion fails.
func (db *DB) SaveOrder(order *models.Order) error {
	query := `INSERT INTO orders (pair, type, side, amount, price, status, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, order.Pair, order.Type, order.Side, order.Amount, order.Price, order.Status, order.Timestamp)
	if err != nil {
		return fmt.Errorf("failed to save order: %v", err)
	}
	return nil
}
