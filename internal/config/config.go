package config

import (
	"os"
	"time"

	"github.com/joho/godotenv"
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
	APIKey    string `yaml:"-"` // YAML에서 제외
	APISecret string `yaml:"-"` // YAML에서 제외
	AccountNo string `yaml:"account_no"`
}

type StrategyConfig struct {
	Name        string  `yaml:"name"`
	ShortPeriod int     `yaml:"short_period"`
	LongPeriod  int     `yaml:"long_period"`
	Threshold   float64 `yaml:"threshold"`
}

func Load(filename string) (*Config, error) {
	// .env 파일 로드
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

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

	// .env 파일에서 로드된 환경 변수에서 API 키와 비밀키 읽기
	config.Exchange.APIKey = os.Getenv("EXCHANGE_API_KEY")
	config.Exchange.APISecret = os.Getenv("EXCHANGE_API_SECRET")

	config.PollingInterval *= time.Minute

	return &config, nil
}
