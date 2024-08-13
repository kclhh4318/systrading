package backtesting

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"tradingbot/internal/config"
	"tradingbot/internal/exchange"
	"tradingbot/internal/strategy"
)

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root")
		}
		dir = parent
	}
}

func TestBacktestingWithMinuteData(t *testing.T) {
	// 프로젝트 루트 디렉토리 찾기
	rootDir, err := findProjectRoot()
	if err != nil {
		t.Fatalf("Failed to find project root: %v", err)
	}
	t.Logf("Project root directory: %s", rootDir)

	// 설정 로드
	configPath := filepath.Join(rootDir, "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Exchange 초기화
	exch, err := exchange.New(cfg.Exchange)
	if err != nil {
		t.Fatalf("Failed to initialize exchange: %v", err)
	}

	// 당일의 1분봉 데이터 가져오기
	historicalData, err := exch.GetMinuteData("041510", 420) // 041510: HD현대중공업, 하루치 1분봉 데이터 (9시~3시)
	if err != nil {
		t.Fatalf("Failed to get minute data: %v", err)
	}

	t.Logf("Retrieved %d minute data points", len(historicalData))

	// Backtester 초기화 및 실행
	strat := strategy.NewMovingAverage(cfg.Strategy)
	backtester := NewBacktester(strat, historicalData, 10000000, 0.0025)
	result := backtester.Run()

	// 결과 검증
	if result.TotalTrades == 0 {
		t.Errorf("Expected some trades, but got 0")
	} else {
		t.Logf("Backtesting Results:")
		t.Logf("Total Trades: %d", result.TotalTrades)
		t.Logf("Winning Trades: %d", result.WinningTrades)
		t.Logf("Losing Trades: %d", result.LosingTrades)
		t.Logf("Total Profit: %.2f", result.TotalProfit)
		t.Logf("Max Drawdown: %.2f%%", result.MaxDrawdown*100)
		t.Logf("Win Rate: %.2f%%", result.WinRate*100)
		t.Logf("Average Profit per Trade: %.2f%%", result.AverageProfitPerTrade)
		t.Logf("Start Date: %v", result.StartDate)
		t.Logf("End Date: %v", result.EndDate)
	}
}
