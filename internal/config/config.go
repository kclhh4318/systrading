package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	DatabaseURL     string         `yaml:"database_url"`
	Exchange        ExchangeConfig `yaml:"exchange"`
	Strategy        StrategyConfig `yaml:"strategy"`
	TradingPair     string         `yaml:"trading_pair"`
	PollingInterval time.Duration  `yaml:"polling_interval"`
}

type ExchangeConfig struct {
	Name      string `yaml:"name"`
	APIKey    string `yaml:"api_key"`
	APISecret string `yaml:"api_secret"`
	AccountNo string `yaml:"account_no"` // 계좌 번호 필드 추가
}

type StrategyConfig struct {
	Name        string  `yaml:"name"`
	ShortPeriod int     `yaml:"short_period"`
	LongPeriod  int     `yaml:"long_period"`
	Threshold   float64 `yaml:"threshold"`
}

func Load(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	config.PollingInterval *= time.Minute // polling_interval을 분 단위로 변환

	return &config, nil
}
