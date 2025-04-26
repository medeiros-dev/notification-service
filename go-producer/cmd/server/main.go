package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/medeiros-dev/notification-service/go-producer/configs"
	_ "github.com/medeiros-dev/notification-service/go-producer/docs" // Import generated docs
	"github.com/medeiros-dev/notification-service/go-producer/internal/infrastructure/broker"
	"github.com/medeiros-dev/notification-service/go-producer/internal/observability/metrics"
	"github.com/medeiros-dev/notification-service/go-producer/internal/observability/tracing"
	"github.com/medeiros-dev/notification-service/go-producer/internal/usecases/sendnotification"
	"github.com/medeiros-dev/notification-service/go-producer/pkg/logger"
	swaggerFiles "github.com/swaggo/files"     // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger" // gin-swagger middleware
	"go.uber.org/zap"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func main() {
	if err := logger.InitializeLogger(false); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			log.Printf("Error syncing logger: %v", err)
		}
	}()

	cfg, err := configs.NewConfig(".")
	if err != nil {
		logger.L().Fatal("Failed to load config", zap.Error(err))
	}

	shutdown, err := tracing.InitTracer(cfg)
	if err != nil {
		logger.L().Fatal("Failed to initialize tracer", zap.Error(err))
	}
	defer func() {
		if err := shutdown(nil); err != nil {
			logger.L().Error("Error shutting down tracer", zap.Error(err))
		}
	}()

	kbrokerCfg := broker.Config{
		Brokers: cfg.KafkaBrokers,
	}
	messageBroker, err := broker.NewKafkaBroker(kbrokerCfg)
	if err != nil {
		logger.L().Fatal("Failed to initialize Kafka broker", zap.Error(err))
	}

	sendNotificationHandler := sendnotification.NewSendNotification(messageBroker)

	metrics.InitMetrics()

	srv := gin.Default()
	srv.Use(otelgin.Middleware(cfg.OtelServiceName))

	// Swagger endpoint
	srv.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	srv.Use(func(c *gin.Context) {
		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = c.Request.URL.Path
		}
		if endpoint == "/metrics" {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		metrics.HttpRequestsTotal.WithLabelValues(endpoint, http.StatusText(status)).Inc()
		metrics.HttpRequestDuration.WithLabelValues(endpoint).Observe(time.Since(start).Seconds())
	})

	srv.POST("/send-notification", sendNotificationHandler.Handle)

	srv.GET("/metrics", gin.WrapH(metrics.MetricsHandler()))

	logger.L().Info("Server starting", zap.String("address", ":8080"))
	if err := srv.Run(":8080"); err != nil {
		logger.L().Fatal("Failed to start server", zap.Error(err))
	}
}
