package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/medeiros-dev/notification-service/consumers/configs"
	"github.com/medeiros-dev/notification-service/consumers/internal/app/registry"
	"github.com/medeiros-dev/notification-service/consumers/internal/infrastructure/broker"
	"github.com/medeiros-dev/notification-service/consumers/internal/observability/metrics"
	"github.com/medeiros-dev/notification-service/consumers/internal/observability/tracing"
	"github.com/medeiros-dev/notification-service/consumers/pkg/logger"
	"go.uber.org/zap"

	// Import channel packages solely for their init() registration effect
	_ "github.com/medeiros-dev/notification-service/consumers/internal/infrastructure/channel/email"
	// _ "github.com/medeiros-dev/notification-service/consumers/internal/infrastructure/channel/sms" // Add future channels here
	"github.com/medeiros-dev/notification-service/consumers/internal/usecases/notification"
	"github.com/medeiros-dev/notification-service/consumers/internal/usecases/queueconsumer"
)

func main() {
	// TODO: Consider making 'isDevelopment' configurable (e.g., via env var)
	if err := logger.InitializeLogger(false); err != nil {
		// Use standard log for the critical error of logger initialization failure
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			// Use standard log as Zap might be unavailable
			log.Printf("Error syncing logger: %v", err)
		}
	}()

	logger.L().Info("Starting notification consumer service...")

	// --- Configuration ---
	cfg, err := configs.NewConfig(".") // Load config from current directory
	if err != nil {
		logger.L().Fatal("Failed to load configuration", zap.Error(err))
	}
	logger.L().Info("Configuration loaded",
		zap.Strings("kafkaBrokers", cfg.KafkaBrokers),
		zap.Strings("enabledChannels", cfg.EnabledChannels),
		zap.String("metricsServerAddress", cfg.MetricsServerAddress),
		zap.String("kafkaTopic", cfg.KafkaTopic),
		zap.String("kafkaGroupID", cfg.KafkaGroupID),
	)

	// --- Initialize OpenTelemetry Tracer ---
	tracerShutdown, err := tracing.InitTracer(cfg)
	if err != nil {
		logger.L().Fatal("Failed to initialize tracer", zap.Error(err))
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracerShutdown(shutdownCtx); err != nil {
			logger.L().Error("Error shutting down tracer provider", zap.Error(err))
		}
	}()

	// --- Start Metrics Server ---
	// Create a new ServeMux for the metrics server to avoid using DefaultServeMux
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metrics.MetricsHandler())

	metricsServer := &http.Server{
		Addr:    cfg.MetricsServerAddress,
		Handler: metricsMux,
	}

	go func() {
		logger.L().Info("Starting metrics server", zap.String("address", cfg.MetricsServerAddress))
		// Use ListenAndServe on the specific server instance
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			logger.L().Error("Metrics server ListenAndServe failed", zap.Error(err))
		}
		// Log moved to after Shutdown() call in main
	}()

	// --- Kafka Broker ---
	kBrokerCfg := broker.Config{Brokers: cfg.KafkaBrokers}
	kmessageBroker, err := broker.NewKafkaBroker(kBrokerCfg)
	if err != nil {
		logger.L().Fatal("Failed to initialize Kafka broker", zap.Error(err))
	}
	// Handle potential close on shutdown
	defer func() {
		if kmessageBroker != nil {
			logger.L().Info("Attempting to close Kafka broker...")
			if err := kmessageBroker.Close(); err != nil {
				logger.L().Error("Error closing kafka broker", zap.Error(err))
			} else {
				logger.L().Info("Kafka broker closed.")
			}
		}
	}()
	logger.L().Info("Kafka Broker initialized")

	// --- Channel Setup (Dynamic via Registry) ---
	channels := make(map[string]*notification.DispatchNotificationHandler)
	logger.L().Info("Initializing enabled channels based on configuration", zap.Strings("channels", cfg.EnabledChannels))
	for _, channelName := range cfg.EnabledChannels {
		logger.L().Debug("Attempting to initialize channel", zap.String("channelName", channelName))
		factory, err := registry.GetChannelFactory(channelName)
		if err != nil {
			logger.L().Warn("No factory registered for channel, skipping.",
				zap.String("channelName", channelName),
				zap.Error(err),
			)
			continue
		}

		channelInstance, err := factory(cfg)
		if err != nil {
			logger.L().Warn("Failed to create channel instance, skipping.",
				zap.String("channelName", channelName),
				zap.Error(err),
			)
			continue
		}

		// Wrap the channel instance with the use case and handler
		handler := notification.NewDispatchNotification(channelInstance)

		channels[channelName] = handler
		logger.L().Info("Successfully initialized and registered handler for channel", zap.String("channelName", channelName))
	}

	if len(channels) == 0 {
		logger.L().Fatal("CRITICAL: No channels were successfully initialized. Check configuration and channel implementations.")
	}

	// --- Graceful Shutdown Setup ---
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// --- Start Consumption (Now happens after setting up shutdown logic) ---
	queueConsumer := queueconsumer.NewQueueConsumer(kmessageBroker, channels, configs.GetQueueConsumerConfig())
	logger.L().Info("Starting Kafka consumer",
		zap.String("topic", cfg.KafkaTopic),
		zap.String("groupID", cfg.KafkaGroupID),
	)

	// Start consumer in a goroutine so main can wait for shutdown signals
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		// Run the consumer loop. This will block until ctx is cancelled.
		if err := queueConsumer.Handle(ctx); err != nil {
			logger.L().Error("Kafka consumer Handle exited with error", zap.Error(err))
		} else {
			logger.L().Info("Kafka consumer Handle exited cleanly.")
		}
	}()

	// Wait for termination signal
	sig := <-sigChan
	logger.L().Info("Received signal, shutting down gracefully...", zap.String("signal", sig.String()))

	// --- Start Shutdown Process ---
	// 1. Shutdown metrics server first (as it's independent)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second) // Give 5s for shutdown
	defer shutdownCancel()
	logger.L().Info("Shutting down metrics server...")
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.L().Error("Metrics server shutdown error", zap.Error(err))
	} else {
		logger.L().Info("Metrics server shut down successfully.")
	}

	// 2. Signal consumer loop to stop *after* initiating metrics server shutdown
	cancel()

	// Wait for consumer to finish after cancellation signal
	logger.L().Info("Waiting for Kafka consumer to stop...")
	<-consumerDone
	logger.L().Info("Kafka consumer stopped.")

	// Kafka broker deferred close will run here

	logger.L().Info("Notification consumer service shut down complete.")
}
