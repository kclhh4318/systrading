package database

import (
	"database/sql"
	"tradingbot/internal/models"

	_ "github.com/go-sql-driver/mysql"
)

type DB struct {
	*sql.DB
}

func NewConnection(databaseURL string) (*DB, error) {
	db, err := sql.Open("mysql", databaseURL)
	if err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

func (db *DB) SaveOrder(order *models.Order) error {
	query := `INSERT INTO orders (pair, type, side, amount, price, status, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, order.Pair, order.Type, order.Side, order.Amount, order.Price, order.Status, order.Timestamp)
	return err
}
