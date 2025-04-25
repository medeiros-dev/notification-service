package email

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"

	"github.com/medeiros-dev/notification-service/consumers/configs"
	"github.com/medeiros-dev/notification-service/consumers/internal/app/registry"
	"github.com/medeiros-dev/notification-service/consumers/internal/domain/port/channel"
	"github.com/medeiros-dev/notification-service/consumers/pkg/logger"
	"go.uber.org/zap"
)

const ChannelName = "email"

// SMTPEmailService implements channel.Channel using SMTP.
type SMTPEmailService struct {
	fromName    string
	fromAddress string
	username    string
	password    string
	smtpHost    string
	smtpPort    string
	auth        smtp.Auth
}

// Factory function for creating SMTPEmailService instances.
func NewSMTPEmailServiceFactory(cfg *configs.Config) (channel.Channel, error) {
	if cfg.EmailHost == "" || cfg.EmailPort == "" || cfg.EmailFromAddress == "" {
		return nil, errors.New("SMTP configuration (host, port, from_address) cannot be empty")
	}
	// Authentication might be optional depending on the SMTP server
	auth := smtp.PlainAuth("", cfg.EmailUsername, cfg.EmailPassword, cfg.EmailHost)

	logger.L().Info("Initializing SMTP Email Service",
		zap.String("host", cfg.EmailHost),
		zap.String("port", cfg.EmailPort),
		zap.String("fromAddress", cfg.EmailFromAddress),  // Log PII carefully
		zap.Bool("authEnabled", cfg.EmailUsername != ""), // Indicate if auth is configured
	)
	return &SMTPEmailService{
		fromName:    cfg.EmailFromName,
		fromAddress: cfg.EmailFromAddress,
		username:    cfg.EmailUsername,
		password:    cfg.EmailPassword,
		smtpHost:    cfg.EmailHost,
		smtpPort:    cfg.EmailPort,
		auth:        auth,
	}, nil
}

// init registers the SMTP email channel factory.
func init() {
	if err := registry.RegisterChannelFactory(ChannelName, NewSMTPEmailServiceFactory); err != nil {
		// Use panic during initialization if registration fails, as it's a programming error.
		panic(fmt.Sprintf("Failed to register channel factory '%s': %v", ChannelName, err))
	}
	logger.L().Info("Channel factory registered", zap.String("channelName", ChannelName))
}

// Send sends an email using the configured SMTP server.
func (s *SMTPEmailService) Send(ctx context.Context, body, to string) error {
	// Retrieve traceID from context if available
	traceID, _ := ctx.Value("traceID").(string) // Assuming traceID is passed via context

	// Use defaults if name/address are empty in config, although factory checks FromAddress
	fromDisplay := s.fromName
	if fromDisplay == "" {
		fromDisplay = "Notification Service"
	}
	from := fmt.Sprintf("%s <%s>", fromDisplay, s.fromAddress)

	// TODO: Make subject configurable or part of the input
	subject := "Payment Notification"
	contentType := "text/html; charset=UTF-8"

	// Construct the email message
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: %s\r\n\r\n%s",
		from, to, subject, contentType, body)

	// Send the email
	smtpAddr := s.smtpHost + ":" + s.smtpPort
	err := smtp.SendMail(smtpAddr, s.auth, s.fromAddress, []string{to}, []byte(msg))
	if err != nil {
		logger.L().Error("Error sending email via SMTP",
			zap.String("recipient", to), // Log PII carefully
			zap.String("smtpHost", s.smtpHost),
			zap.String("fromAddress", s.fromAddress), // Log PII carefully
			zap.String("traceID", traceID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to send email via SMTP to %s: %w", to, err)
	}
	logger.L().Info("Email sent successfully via SMTP",
		zap.String("recipient", to), // Log PII carefully
		zap.String("smtpHost", s.smtpHost),
		zap.String("fromAddress", s.fromAddress), // Log PII carefully
		zap.String("traceID", traceID),
	)
	return nil
}
