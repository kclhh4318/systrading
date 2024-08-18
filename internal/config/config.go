package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"tradingbot/internal/models"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v2"
)

type Config struct {
	DatabaseURL     string                `yaml:"database_url"`
	Exchange        ExchangeConfig        `yaml:"exchange"`
	TradingPair     string                `yaml:"trading_pair"`
	PollingInterval string                `yaml:"polling_interval"`
	ParsedInterval  time.Duration         `yaml:"-"`
	Strategy        models.StrategyConfig `yaml:"strategy"`
}

type ExchangeConfig struct {
	Name        string `yaml:"name"`
	AccountNo   string `yaml:"account_no"`
	AppKey      string `yaml:"-"`
	AppSecret   string `yaml:"-"`
	AccessToken string `yaml:"-"`
}

func Load(filename string) (*Config, error) {
	envPath := filepath.Join(filepath.Dir(filename), ".env")
	err := godotenv.Load(envPath)
	if err != nil {
		fmt.Printf("Warning: Error loading .env file: %v\n", err)
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %v", err)
	}
	defer file.Close()

	var config Config
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config file: %v", err)
	}

	config.Exchange.AppKey = os.Getenv("EXCHANGE_API_KEY")
	config.Exchange.AppSecret = os.Getenv("EXCHANGE_API_SECRET")

	duration, err := time.ParseDuration(config.PollingInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to parse polling interval: %v", err)
	}
	config.ParsedInterval = duration

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Config) Validate() error {
	if c.Strategy.ShortPeriod <= 0 || c.Strategy.LongPeriod <= 0 {
		return fmt.Errorf("strategy periods must be positive")
	}
	if c.Strategy.ShortPeriod >= c.Strategy.LongPeriod {
		return fmt.Errorf("short period must be less than long period")
	}
	return nil
}
