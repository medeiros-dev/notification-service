package configs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	KafkaBrokers    []string `mapstructure:"KAFKA_BROKERS"`
	KafkaTopic      string   `mapstructure:"KAFKA_TOPIC"`
	EnabledChannels []string `mapstructure:"ENABLED_CHANNELS"`
	OtelEndpoint    string   `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	OtelServiceName string   `mapstructure:"OTEL_SERVICE_NAME"`
	OtelInsecure    bool     `mapstructure:"OTEL_EXPORTER_OTLP_INSECURE"`
}

var cfg *Config

func NewConfig(path string) (*Config, error) {
	relativeUrl, err := GetBasePath(path)
	if err != nil {
		return nil, fmt.Errorf("error getting base path: %v", err)
	}

	vip := viper.New()
	vip.SetConfigType("env")
	vip.SetConfigName(".env")
	vip.AddConfigPath(relativeUrl)
	vip.AutomaticEnv()

	if err := vip.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	vip.BindEnv("KAFKA_BROKERS")
	vip.BindEnv("KAFKA_TOPIC")
	vip.BindEnv("ENABLED_CHANNELS")
	vip.BindEnv("OTEL_EXPORTER_OTLP_ENDPOINT")
	vip.BindEnv("OTEL_SERVICE_NAME")
	vip.BindEnv("OTEL_EXPORTER_OTLP_INSECURE")

	if err := vip.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %v", err)
	}

	if !vip.IsSet("OTEL_EXPORTER_OTLP_INSECURE") {
		cfg.OtelInsecure = false
	}
	return cfg, nil
}

func GetBasePath(path string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return filepath.Join(cwd, path), nil
		}

		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", errors.New("go.mod not found")
		}
		cwd = parent
	}
}

func GetConfig() *Config {
	return cfg
}

func SetConfig(newCfg *Config) {
	cfg = newCfg
}
