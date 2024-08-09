package strategy

import (
	"tradingbot/internal/config"
	"tradingbot/internal/models"
)

const (
	BuySignal  = "buy"
	SellSignal = "sell"
	HoldSignal = "hold"
)

type Strategy interface {
	Analyze(data *models.MarketData) *models.Signal
}

type MovingAverage struct {
	ShortPeriod int
	LongPeriod  int
	Threshold   float64
}

func NewMovingAverage(config config.StrategyConfig) *MovingAverage {
	return &MovingAverage{
		ShortPeriod: config.ShortPeriod,
		LongPeriod:  config.LongPeriod,
		Threshold:   config.Threshold,
	}
}

func (ma *MovingAverage) Analyze(data *models.MarketData) *models.Signal {
	// Implement the moving average analysis logic
	return &models.Signal{
		Type: HoldSignal,
	}
}
