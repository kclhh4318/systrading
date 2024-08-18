package models

type StrategyConfig struct {
	ShortPeriod int     `yaml:"short_period"`
	LongPeriod  int     `yaml:"long_period"`
	Threshold   float64 `yaml:"threshold"`
}
