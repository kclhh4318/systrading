package backtesting

import (
	"strconv"
	"time"
	"tradingbot/internal/models"
	"tradingbot/internal/strategy"
)

type BacktestResult struct {
	TotalTrades           int
	WinningTrades         int
	LosingTrades          int
	TotalProfit           float64
	MaxDrawdown           float64
	WinRate               float64
	AverageProfitPerTrade float64
	StartDate             time.Time
	EndDate               time.Time
}

type Backtester struct {
	Strategy       strategy.Strategy
	Data           []models.MarketData
	InitialBalance float64
	CommissionRate float64
}

func NewBacktester(strat strategy.Strategy, data []models.MarketData, initialBalance, commissionRate float64) *Backtester {
	return &Backtester{
		Strategy:       strat,
		Data:           data,
		InitialBalance: initialBalance,
		CommissionRate: commissionRate,
	}
}

func (b *Backtester) Run() BacktestResult {

	balance := b.InitialBalance
	position := 0.0
	entryPrice := 0.0
	result := BacktestResult{
		StartDate: time.Now().AddDate(0, 0, -len(b.Data)),
		EndDate:   time.Now(),
	}
	maxBalance := balance

	for _, data := range b.Data { // Removed `i` since it's not used
		signal := b.Strategy.Analyze(&data)
		currentPrice := parsePrice(data.StckPrpr)

		switch signal.Type {
		case strategy.BuySignal:
			if position == 0 {
				position = (balance * (1 - b.CommissionRate)) / currentPrice
				entryPrice = currentPrice
				balance = 0
				result.TotalTrades++
			}
		case strategy.SellSignal:
			if position > 0 {
				balance = position * currentPrice * (1 - b.CommissionRate)
				profit := balance - b.InitialBalance
				result.TotalTrades++
				if profit > 0 {
					result.WinningTrades++
				} else {
					result.LosingTrades++
				}
				result.TotalProfit += profit

				profitPercentage := (currentPrice - entryPrice) / entryPrice * 100
				result.AverageProfitPerTrade += profitPercentage

				position = 0
				entryPrice = 0
			}
		}

		currentBalance := balance
		if position > 0 {
			currentBalance = position * currentPrice
		}
		if currentBalance > maxBalance {
			maxBalance = currentBalance
		}
		drawdown := (maxBalance - currentBalance) / maxBalance
		if drawdown > result.MaxDrawdown {
			result.MaxDrawdown = drawdown
		}
	}

	// 마지막 포지션 청산
	if position > 0 {
		finalPrice := parsePrice(b.Data[len(b.Data)-1].StckPrpr)
		balance = position * finalPrice * (1 - b.CommissionRate)
		profit := balance - b.InitialBalance
		result.TotalProfit += profit
		result.TotalTrades++
		if profit > 0 {
			result.WinningTrades++
		} else {
			result.LosingTrades++
		}

		profitPercentage := (finalPrice - entryPrice) / entryPrice * 100
		result.AverageProfitPerTrade += profitPercentage
	}

	if result.TotalTrades > 0 {
		result.WinRate = float64(result.WinningTrades) / float64(result.TotalTrades)
		result.AverageProfitPerTrade /= float64(result.TotalTrades)
	}

	return result
}

func parsePrice(priceStr string) float64 {
	price, _ := strconv.ParseFloat(priceStr, 64)
	return price
}
