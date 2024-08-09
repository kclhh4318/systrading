package models

import "time"

type Order struct {
	ID        int64
	Pair      string
	Type      string
	Side      string
	Amount    float64
	Price     float64
	Status    string
	Timestamp time.Time
}

// Similar structures for Trade and MarketData in their respective files
