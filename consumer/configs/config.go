package configs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	MaxRetries           int      `mapstructure:"MAX_RETRIES"`
	EnabledChannels      []string `mapstructure:"ENABLED_CHANNELS"`
	WorkerPoolSize       int      `mapstructure:"WORKER_POOL_SIZE"`
	BackoffBaseDelay     int      `mapstructure:"BACKOFF_BASE_DELAY_MS"`
	KafkaBrokers         []string `mapstructure:"KAFKA_BROKERS"`
	KafkaTopic           string   `mapstructure:"KAFKA_TOPIC"`
	KafkaGroupID         string   `mapstructure:"KAFKA_GROUP_ID"`
	KafkaDLQTopic        string   `mapstructure:"KAFKA_DLQ_TOPIC"`
	EmailDriver          string   `mapstructure:"EMAIL_DRIVER"`
	EmailMailer          string   `mapstructure:"EMAIL_MAILER"`
	EmailHost            string   `mapstructure:"EMAIL_HOST"`
	EmailPort            string   `mapstructure:"EMAIL_PORT"`
	EmailUsername        string   `mapstructure:"EMAIL_USERNAME"`
	EmailPassword        string   `mapstructure:"EMAIL_PASSWORD"`
	EmailEncrypt         string   `mapstructure:"EMAIL_ENCRYPTION"`
	EmailFromAddress     string   `mapstructure:"EMAIL_FROM_ADDRESS"`
	EmailFromName        string   `mapstructure:"EMAIL_FROM_NAME"`
	MetricsServerAddress string   `mapstructure:"METRICS_SERVER_ADDRESS"`
	OtelEndpoint         string   `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	OtelInsecure         bool     `mapstructure:"OTEL_EXPORTER_OTLP_INSECURE"`
	OtelServiceName      string   `mapstructure:"OTEL_SERVICE_NAME"`
}

type EmailConf struct {
	Driver      string
	Mailer      string
	Host        string
	Port        string
	Username    string
	Password    string
	Encrypt     string
	FromAddress string
	FromName    string
}

type QueueConsumerConfig struct {
	MaxRetries       int
	EnabledChannels  []string
	WorkerPoolSize   int
	BackoffBaseDelay int
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

	vip.BindEnv("MAX_RETRIES")
	vip.BindEnv("ENABLED_CHANNELS")
	vip.BindEnv("WORKER_POOL_SIZE")
	vip.BindEnv("BACKOFF_BASE_DELAY_MS")
	vip.BindEnv("METRICS_SERVER_ADDRESS")
	vip.BindEnv("OTEL_EXPORTER_OTLP_ENDPOINT")
	vip.BindEnv("OTEL_EXPORTER_OTLP_INSECURE")
	vip.BindEnv("OTEL_SERVICE_NAME")
	vip.BindEnv("KAFKA_BROKERS")
	vip.BindEnv("KAFKA_TOPIC")
	vip.BindEnv("KAFKA_GROUP_ID")
	vip.BindEnv("KAFKA_DLQ_TOPIC")
	vip.BindEnv("EMAIL_DRIVER")
	vip.BindEnv("EMAIL_MAILER")
	vip.BindEnv("EMAIL_HOST")
	vip.BindEnv("EMAIL_PORT")
	vip.BindEnv("EMAIL_USERNAME")
	vip.BindEnv("EMAIL_PASSWORD")
	vip.BindEnv("EMAIL_ENCRYPTION")
	vip.BindEnv("EMAIL_FROM_ADDRESS")
	vip.BindEnv("EMAIL_FROM_NAME")

	if err := vip.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %v", err)
	}

	if !vip.IsSet("otel_exporter_otlp_insecure") {
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

func GetEmailConf() *EmailConf {

	if cfg == nil {
		return &EmailConf{}
	}

	return &EmailConf{
		Driver:      cfg.EmailDriver,
		Mailer:      cfg.EmailMailer,
		Host:        cfg.EmailHost,
		Port:        cfg.EmailPort,
		Username:    cfg.EmailUsername,
		Password:    cfg.EmailPassword,
		Encrypt:     cfg.EmailEncrypt,
		FromAddress: cfg.EmailFromAddress,
		FromName:    cfg.EmailFromName,
	}
}

func GetQueueConsumerConfig() *QueueConsumerConfig {
	return &QueueConsumerConfig{
		MaxRetries:       cfg.MaxRetries,
		EnabledChannels:  cfg.EnabledChannels,
		WorkerPoolSize:   cfg.WorkerPoolSize,
		BackoffBaseDelay: cfg.BackoffBaseDelay,
	}
}

// SetTestConfig allows tests to set the global config variable directly.
func SetTestConfig(testCfg *Config) {
	cfg = testCfg
}
