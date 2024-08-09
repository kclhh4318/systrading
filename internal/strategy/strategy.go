package strategy

import (
	"log"
	"strconv"
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
	ShortPeriod  int
	LongPeriod   int
	Threshold    float64
	PriceHistory []float64
}

func NewMovingAverage(config config.StrategyConfig) *MovingAverage {
	return &MovingAverage{
		ShortPeriod:  config.ShortPeriod,
		LongPeriod:   config.LongPeriod,
		Threshold:    config.Threshold,
		PriceHistory: []float64{},
	}
}

func (ma *MovingAverage) Analyze(data *models.MarketData) *models.Signal {
	price, err := strconv.ParseFloat(data.StckPrpr, 64)
	if err != nil {
		log.Printf("Error parsing price: %v", err)
		return &models.Signal{Type: HoldSignal}
	}

	ma.PriceHistory = append(ma.PriceHistory, price)
	if len(ma.PriceHistory) > ma.LongPeriod {
		ma.PriceHistory = ma.PriceHistory[1:]
	}

	if len(ma.PriceHistory) < ma.LongPeriod {
		log.Printf("Not enough data for analysis. Current data points: %d, Required: %d", len(ma.PriceHistory), ma.LongPeriod)
		return &models.Signal{Type: HoldSignal}
	}

	shortMA := ma.calculateMA(ma.ShortPeriod)
	longMA := ma.calculateMA(ma.LongPeriod)

	log.Printf("Current price: %.2f, Short MA: %.2f, Long MA: %.2f", price, shortMA, longMA)

	if shortMA > longMA*(1+ma.Threshold) {
		return &models.Signal{
			Type:   BuySignal,
			Amount: 1.0, // This is a placeholder. In a real scenario, you'd calculate the amount to buy.
		}
	} else if shortMA < longMA*(1-ma.Threshold) {
		return &models.Signal{
			Type:   SellSignal,
			Amount: 1.0, // This is a placeholder. In a real scenario, you'd calculate the amount to sell.
		}
	}

	return &models.Signal{Type: HoldSignal}
}

func (ma *MovingAverage) calculateMA(period int) float64 {
	if len(ma.PriceHistory) < period {
		return 0
	}

	sum := 0.0
	for i := len(ma.PriceHistory) - period; i < len(ma.PriceHistory); i++ {
		sum += ma.PriceHistory[i]
	}

	return sum / float64(period)
}
