package backtesting

import (
	"fmt"
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

	for _, data := range b.Data {
		signal := b.Strategy.Analyze(&data)
		currentPrice, err := parsePrice(data.StckPrpr)
		if err != nil {
			// 에러 로그 처리 추가 필요
			continue
		}

		switch signal.Type {
		case strategy.BuySignal:
			if position == 0 {
				position, balance = b.executeBuy(balance, currentPrice)
				entryPrice = currentPrice
				result.TotalTrades++
			}
		case strategy.SellSignal:
			if position > 0 {
				balance = b.executeSell(position, currentPrice)
				balance = b.closePosition(currentPrice, entryPrice, &result)
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
		finalPrice, err := parsePrice(b.Data[len(b.Data)-1].StckPrpr)
		if err == nil {
			balance = b.closePosition(finalPrice, entryPrice, &result)
		}
	}

	if result.TotalTrades > 0 {
		result.WinRate = float64(result.WinningTrades) / float64(result.TotalTrades)
		result.AverageProfitPerTrade /= float64(result.TotalTrades)
	}

	return result
}

func parsePrice(priceStr string) (float64, error) {
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse price: %v", err)
	}
	return price, nil
}

func (b *Backtester) closePosition(finalPrice, entryPrice float64, result *BacktestResult) float64 {
	balance := b.InitialBalance * finalPrice / entryPrice
	profit := balance - b.InitialBalance
	result.TotalProfit += profit
	result.TotalTrades++
	if profit > 0 {
		result.WinningTrades++
	} else {
		result.LosingTrades++
	}
	result.AverageProfitPerTrade += (finalPrice - entryPrice) / entryPrice * 100
	return balance
}

func (b *Backtester) executeBuy(balance, currentPrice float64) (float64, float64) {
	position := (balance * (1 - b.CommissionRate)) / currentPrice
	return position, 0 // 포지션을 열고, 잔고를 0으로 설정
}

func (b *Backtester) executeSell(position, currentPrice float64) float64 {
	return position * currentPrice * (1 - b.CommissionRate) // 포지션을 닫고 잔고 갱신
}
