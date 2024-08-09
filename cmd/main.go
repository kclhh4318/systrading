package main

import (
	"log"
	"time"
	"tradingbot/internal/config"
	"tradingbot/internal/database"
	"tradingbot/internal/exchange"
	"tradingbot/internal/strategy"
)

func main() {
	log.Println("Starting trading bot...")

	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Println("Config loaded successfully")

	db, err := database.NewConnection(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Database connection established")

	exch, err := exchange.New(cfg.Exchange)
	if err != nil {
		log.Fatalf("Failed to initialize exchange: %v", err)
	}
	log.Println("Exchange initialized")

	strat := strategy.NewMovingAverage(cfg.Strategy)
	log.Println("Strategy initialized")

	// 삼성전자 주식 현재가 조회
	marketData, err := exch.GetSamsungPrice()
	if err != nil {
		log.Fatalf("Failed to get Samsung price: %v", err)
	}
	log.Printf("Samsung Electronics Stock Price: %s", marketData.StckPrpr)

	// 예수금 조회
	balance, err := exch.GetBalance()
	if err != nil {
		log.Fatalf("Failed to get account balance: %v", err)
	}
	log.Printf("Account Balance: %+v", balance)

	log.Println("Entering main loop...")
	for {
		log.Println("Fetching market data...")
		marketData, err := exch.GetMarketData(cfg.TradingPair)
		if err != nil {
			log.Printf("Failed to get market data: %v", err)
			continue
		}

		log.Println("Analyzing market data...")
		signal := strat.Analyze(marketData)

		if signal.Type != strategy.HoldSignal {
			log.Printf("Signal generated: %s", signal.Type)
			order, err := exch.PlaceOrder(signal)
			if err != nil {
				log.Printf("Failed to place order: %v", err)
			} else {
				log.Printf("Order placed: %+v", order)
				err = database.SaveOrder(db, order)
				if err != nil {
					log.Printf("Failed to save order: %v", err)
				}
			}
		} else {
			log.Println("No trading signal generated")
		}

		log.Printf("Sleeping for %v...", cfg.PollingInterval)
		time.Sleep(cfg.PollingInterval)
	}
}
