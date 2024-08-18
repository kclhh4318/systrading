package models

import "time"

type OrderType string
type OrderSide string
type OrderStatus string

const (
	OrderTypeLimit  OrderType = "limit"
	OrderTypeMarket OrderType = "market"

	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"

	OrderStatusOpen     OrderStatus = "open"
	OrderStatusClosed   OrderStatus = "closed"
	OrderStatusCanceled OrderStatus = "canceled"
)

type Order struct {
	ID        int64       `json:"id" db:"id"`
	Pair      string      `json:"pair" db:"pair"`
	Type      OrderType   `json:"type" db:"type"`
	Side      OrderSide   `json:"side" db:"side"`
	Amount    float64     `json:"amount" db:"amount"`
	Price     float64     `json:"price" db:"price"`
	Status    OrderStatus `json:"status" db:"status"`
	Timestamp time.Time   `json:"timestamp" db:"timestamp"`
}
