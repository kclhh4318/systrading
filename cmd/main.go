package main

import (
	"os"
	"time"
	"tradingbot/internal/backtesting"
	"tradingbot/internal/config"
	"tradingbot/internal/database"
	"tradingbot/internal/exchange"
	"tradingbot/internal/strategy"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("panic", r).Error("Recovered from panic")
		}
	}()

	log.Info("Starting trading bot...")

	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.WithError(err).Fatal("Failed to load config")

	}

	// Run backtesting
	runBacktest(cfg)

	db, err := database.NewConnection(cfg.DatabaseURL)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	exch, err := exchange.New(cfg.Exchange)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize exchange")
	}

	strat := strategy.NewMovingAverage(cfg.Strategy)

	// Initial market check
	marketData, err := exch.GetSamsungPrice()
	if err != nil {
		log.WithError(err).Error("Failed to get Samsung price")
	} else {
		log.WithField("price", marketData.StckPrpr).Info("Samsung Electronics Stock Price")
	}

	// Initial balance check
	balance, err := exch.GetBalance()
	if err != nil {
		log.WithError(err).Error("Failed to get account balance")
	} else {
		log.WithField("balance", balance).Info("Account Balance")
	}

	log.Info("Entering main loop...")
	for {
		if err := runTradingCycle(cfg, exch, strat, db); err != nil {
			log.WithError(err).Error("Error in trading cycle")
		}

		log.WithField("interval", cfg.ParsedInterval).Info("Sleeping")
		time.Sleep(cfg.ParsedInterval)
	}
}

func runTradingCycle(cfg *config.Config, exch *exchange.KISExchange, strat *strategy.MovingAverage, db *database.DB) error {
	marketData, err := exch.GetMarketData(cfg.TradingPair)
	if err != nil {
		return errors.Wrap(err, "failed to get market data")
	}

	signal := strat.Analyze(marketData)
	log.WithField("signal", signal.Type).Info("Strategy analysis result")

	if signal.Type != strategy.HoldSignal {
		log.WithFields(logrus.Fields{
			"type":   signal.Type,
			"amount": signal.Amount,
		}).Info("Signal generated")

		order, err := exch.PlaceOrder(signal)
		if err != nil {
			return errors.Wrap(err, "failed to place order")
		}

		log.WithField("order", order).Info("Order placed")

		if err := db.SaveOrder(order); err != nil {
			return errors.Wrap(err, "failed to save order")
		}
	} else {
		log.Info("No trading action needed")
	}

	return nil
}

// main.go
func runBacktest(cfg *config.Config) {
	log.Info("Starting backtesting...")

	exch, err := exchange.New(cfg.Exchange)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize exchange")
	}

	stockCode := "041510"
	days := 100 // 100일 데이터

	historicalData, err := exch.GetHistoricalData(stockCode, days)
	if err != nil {
		log.WithError(err).Fatal("Failed to get historical data")
	}

	strat := strategy.NewMovingAverage(cfg.Strategy)

	backtester := backtesting.NewBacktester(strat, historicalData, 10000000, 0.0025)

	result := backtester.Run()

	log.WithFields(logrus.Fields{
		"TotalTrades":       result.TotalTrades,
		"WinningTrades":     result.WinningTrades,
		"LosingTrades":      result.LosingTrades,
		"TotalProfit":       result.TotalProfit,
		"MaxDrawdown":       result.MaxDrawdown * 100,
		"WinRate":           result.WinRate * 100,
		"AvgProfitPerTrade": result.AverageProfitPerTrade,
	}).Info("Backtesting results")
}
