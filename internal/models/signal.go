package models

type SignalType string

const (
	BuySignal  SignalType = "buy"
	SellSignal SignalType = "sell"
	HoldSignal SignalType = "hold"
)

type Signal struct {
	Type   SignalType `json:"type"`
	Pair   string     `json:"pair"`
	Amount float64    `json:"amount"`
}
