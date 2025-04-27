package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

func init() {
	// Default to production logger
	err := InitializeLogger(false)
	if err != nil {
		// Use standard library logger for initialization errors if log is nil
		if log != nil {
			zap.NewStdLog(log).Fatalf("Failed to initialize logger: %v", err)
		} else {
			// Fallback: use standard library logger
			os.Stderr.WriteString("Failed to initialize logger: " + err.Error() + "\n")
		}
	}
}

// InitializeLogger sets up the global zap logger.
// isDevelopment determines if the logger should use development-friendly settings.
func InitializeLogger(isDevelopment bool) error {
	var config zap.Config
	if isDevelopment {
		config = zap.NewDevelopmentConfig()
		// Customize development logging if needed
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		// Ensure JSON output for production
		config.Encoding = "json"
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.EncoderConfig.MessageKey = "message"
		config.EncoderConfig.LevelKey = "level"
		config.EncoderConfig.CallerKey = "caller"
		config.EncoderConfig.StacktraceKey = "stacktrace"
	}

	// Set level based on environment or config if needed, default is Info
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel != "" {
		var level zapcore.Level
		if err := level.Set(logLevel); err == nil {
			config.Level.SetLevel(level)
		} else {
			if log != nil {
				zap.NewStdLog(log).Printf("Invalid LOG_LEVEL '%s', using default Info level", logLevel)
			}
		}
	} else {
		config.Level.SetLevel(zap.InfoLevel)
	}

	var err error
	log, err = config.Build(zap.AddCallerSkip(1)) // AddCallerSkip(1) to show the correct caller
	if err != nil {
		log = zap.NewNop()
		return err
	}

	// Redirect standard log package to Zap
	zap.RedirectStdLog(log)

	return nil
}

// L returns the global logger instance.
func L() *zap.Logger {
	return log
}

// Sync flushes any buffered log entries.
func Sync() error {
	if log != nil {
		return log.Sync()
	}
	return nil
}
