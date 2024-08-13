package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v2"
)

type Config struct {
	DatabaseURL     string         `yaml:"database_url"`
	Exchange        ExchangeConfig `yaml:"exchange"`
	Strategy        StrategyConfig `yaml:"strategy"`
	TradingPair     string         `yaml:"trading_pair"`
	PollingInterval string         `yaml:"polling_interval"`
	ParsedInterval  time.Duration  // 파싱된 duration을 저장할 새 필드
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
	// .env 파일 로드 시도 (config.yaml과 같은 디렉토리에서)
	envPath := filepath.Join(filepath.Dir(filename), ".env")
	err := godotenv.Load(envPath)
	if err != nil {
		fmt.Printf("Warning: Error loading .env file: %v\n", err)
	}

	// config.yaml 파일 열기
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

	// 환경 변수에서 API 키와 비밀키 읽기
	config.Exchange.APIKey = os.Getenv("EXCHANGE_API_KEY")
	config.Exchange.APISecret = os.Getenv("EXCHANGE_API_SECRET")

	// API 키와 비밀키가 비어있는지 확인
	if config.Exchange.APIKey == "" || config.Exchange.APISecret == "" {
		fmt.Println("Warning: API key or secret is empty")
	}

	// PollingInterval 파싱
	duration, err := time.ParseDuration(config.PollingInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to parse polling interval: %v", err)
	}
	config.ParsedInterval = duration

	return &config, nil
}
