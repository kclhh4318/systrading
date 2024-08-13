package strategy

import (
	"log"
	"strconv"
	"testing"
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
	ShortSMA     float64 // 추가된 필드
	LongSMA      float64 // 추가된 필드
	PriceHistory []float64
}

func NewMovingAverage(config config.StrategyConfig) *MovingAverage {
	return &MovingAverage{
		ShortPeriod:  5,
		LongPeriod:   10,
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

	// PriceHistory가 LongPeriod보다 길어질 경우 초과된 데이터를 제거
	if len(ma.PriceHistory) > ma.LongPeriod {
		ma.PriceHistory = ma.PriceHistory[1:]
	}

	// 충분한 데이터가 없으면 Hold 신호를 반환
	if len(ma.PriceHistory) < ma.LongPeriod {
		log.Printf("Not enough data to calculate moving averages. Data points: %d", len(ma.PriceHistory))
		return &models.Signal{Type: HoldSignal}
	}

	ma.updateSMA()

	// 이동 평균 로그 추가
	log.Printf("ShortSMA: %.2f, LongSMA: %.2f", ma.ShortSMA, ma.LongSMA)

	if ma.ShortSMA > ma.LongSMA*(1+ma.Threshold) {
		log.Printf("Buy signal triggered. ShortSMA: %.2f > LongSMA: %.2f * (1 + %.2f)", ma.ShortSMA, ma.LongSMA, ma.Threshold)
		return &models.Signal{Type: BuySignal, Amount: 1.0}
	} else if ma.ShortSMA < ma.LongSMA*(1-ma.Threshold) {
		log.Printf("Sell signal triggered. ShortSMA: %.2f < LongSMA: %.2f * (1 - %.2f)", ma.ShortSMA, ma.LongSMA, ma.Threshold)
		return &models.Signal{Type: SellSignal, Amount: 1.0}
	}

	log.Printf("Hold signal triggered. ShortSMA: %.2f, LongSMA: %.2f", ma.ShortSMA, ma.LongSMA)
	return &models.Signal{Type: HoldSignal}
}

func (ma *MovingAverage) updateSMA() {
	ma.ShortSMA = ma.calculateSMA(ma.ShortPeriod)
	ma.LongSMA = ma.calculateSMA(ma.LongPeriod)
}

func (ma *MovingAverage) calculateSMA(period int) float64 {
	if len(ma.PriceHistory) < period {
		return 0.0
	}

	sum := 0.0
	for i := len(ma.PriceHistory) - period; i < len(ma.PriceHistory); i++ {
		sum += ma.PriceHistory[i]
	}

	return sum / float64(period)
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

func TestMovingAverageAnalyze(t *testing.T) {
	ma := NewMovingAverage(config.StrategyConfig{
		ShortPeriod: 5,
		LongPeriod:  10,
		Threshold:   0.01,
	})

	testCases := []struct {
		name     string
		prices   []string
		expected string
	}{
		{
			name:     "Uptrend",
			prices:   []string{"100", "101", "102", "103", "104", "105", "106", "107", "108", "109", "110"},
			expected: BuySignal,
		},
		{
			name:     "Downtrend",
			prices:   []string{"110", "109", "108", "107", "106", "105", "104", "103", "102", "101", "100"},
			expected: SellSignal,
		},
		{
			name:     "Sideways",
			prices:   []string{"100", "101", "100", "101", "100", "101", "100", "101", "100", "101", "100"},
			expected: HoldSignal,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, price := range tc.prices {
				ma.Analyze(&models.MarketData{StckPrpr: price})
			}
			result := ma.Analyze(&models.MarketData{StckPrpr: tc.prices[len(tc.prices)-1]})
			if result.Type != tc.expected {
				t.Errorf("Expected %s, but got %s", tc.expected, result.Type)
			}
		})
	}
}
