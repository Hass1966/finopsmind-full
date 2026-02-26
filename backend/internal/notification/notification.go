// Package notification provides multi-channel notification delivery.
package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

// Channel represents a notification delivery channel.
type Channel string

const (
	ChannelSlack   Channel = "slack"
	ChannelEmail   Channel = "email"
	ChannelWebhook Channel = "webhook"
)

// EventType represents the type of notification event.
type EventType string

const (
	EventAnomalyDetected    EventType = "anomaly.detected"
	EventBudgetExceeded     EventType = "budget.exceeded"
	EventBudgetWarning      EventType = "budget.warning"
	EventRecommendationNew  EventType = "recommendation.new"
	EventCostSpike          EventType = "cost.spike"
	EventWeeklyDigest       EventType = "weekly.digest"
)

// Message represents a notification message.
type Message struct {
	EventType EventType              `json:"event_type"`
	Title     string                 `json:"title"`
	Body      string                 `json:"body"`
	Severity  string                 `json:"severity,omitempty"`
	Data      map[string]any         `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// Config holds notification service configuration.
type Config struct {
	SlackWebhookURL string
	EmailSMTPHost   string
	EmailSMTPPort   int
	EmailFrom       string
	EmailPassword   string
	EmailRecipients []string // org-specific recipients
	WebhookURLs     []string
}

// Service manages notification delivery across channels.
type Service struct {
	cfg        Config
	httpClient *http.Client
	logger     *slog.Logger
	channels   []Channel
}

// NewService creates a new notification service.
func NewService(cfg Config, logger *slog.Logger) *Service {
	s := &Service{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}

	if cfg.SlackWebhookURL != "" {
		s.channels = append(s.channels, ChannelSlack)
	}
	if cfg.EmailSMTPHost != "" {
		s.channels = append(s.channels, ChannelEmail)
	}
	if len(cfg.WebhookURLs) > 0 {
		s.channels = append(s.channels, ChannelWebhook)
	}

	return s
}

// Send sends a notification to all configured channels.
func (s *Service) Send(ctx context.Context, msg Message) error {
	msg.Timestamp = time.Now().UTC()
	var errs []string

	for _, ch := range s.channels {
		var err error
		switch ch {
		case ChannelSlack:
			err = s.sendSlack(ctx, msg)
		case ChannelEmail:
			err = s.sendEmail(ctx, msg)
		case ChannelWebhook:
			err = s.sendWebhook(ctx, msg)
		}
		if err != nil {
			s.logger.Error("notification send failed", "channel", ch, "event", msg.EventType, "error", err)
			errs = append(errs, fmt.Sprintf("%s: %v", ch, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("notification errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// SendToChannel sends a notification to a specific channel.
func (s *Service) SendToChannel(ctx context.Context, ch Channel, msg Message) error {
	msg.Timestamp = time.Now().UTC()
	switch ch {
	case ChannelSlack:
		return s.sendSlack(ctx, msg)
	case ChannelEmail:
		return s.sendEmail(ctx, msg)
	case ChannelWebhook:
		return s.sendWebhook(ctx, msg)
	default:
		return fmt.Errorf("unsupported channel: %s", ch)
	}
}

// HasChannel returns true if the specified channel is configured.
func (s *Service) HasChannel(ch Channel) bool {
	for _, c := range s.channels {
		if c == ch {
			return true
		}
	}
	return false
}

func (s *Service) sendSlack(ctx context.Context, msg Message) error {
	color := "#2196F3" // blue
	switch msg.Severity {
	case "critical":
		color = "#FF0000"
	case "high":
		color = "#FF9800"
	case "medium":
		color = "#FFC107"
	}

	payload := map[string]any{
		"attachments": []map[string]any{
			{
				"color":  color,
				"title":  msg.Title,
				"text":   msg.Body,
				"footer": "FinOpsMind",
				"ts":     msg.Timestamp.Unix(),
				"fields": buildSlackFields(msg.Data),
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.SlackWebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	s.logger.Info("slack notification sent", "event", msg.EventType)
	return nil
}

func (s *Service) sendEmail(ctx context.Context, msg Message) error {
	if s.cfg.EmailSMTPHost == "" {
		return fmt.Errorf("email SMTP not configured")
	}

	subject := fmt.Sprintf("[FinOpsMind] %s", msg.Title)
	body := fmt.Sprintf("Subject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n\r\nEvent: %s\r\nTime: %s",
		subject, msg.Body, msg.EventType, msg.Timestamp.Format(time.RFC3339))

	addr := fmt.Sprintf("%s:%d", s.cfg.EmailSMTPHost, s.cfg.EmailSMTPPort)

	var auth smtp.Auth
	if s.cfg.EmailPassword != "" {
		auth = smtp.PlainAuth("", s.cfg.EmailFrom, s.cfg.EmailPassword, s.cfg.EmailSMTPHost)
	}

	// Send to configured recipients, or fall back to from address
	recipients := s.cfg.EmailRecipients
	if len(recipients) == 0 {
		recipients = []string{s.cfg.EmailFrom}
	}

	err := smtp.SendMail(addr, auth, s.cfg.EmailFrom, recipients, []byte(body))
	if err != nil {
		return fmt.Errorf("email send failed: %w", err)
	}

	s.logger.Info("email notification sent", "event", msg.EventType)
	return nil
}

func (s *Service) sendWebhook(ctx context.Context, msg Message) error {
	body, _ := json.Marshal(msg)

	var errs []string
	for _, webhookURL := range s.cfg.WebhookURLs {
		req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(body))
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-FinOpsMind-Event", string(msg.EventType))

		resp, err := s.httpClient.Do(req)
		if err != nil {
			errs = append(errs, fmt.Sprintf("webhook %s: %v", webhookURL, err))
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 300 {
			errs = append(errs, fmt.Sprintf("webhook %s: status %d", webhookURL, resp.StatusCode))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("webhook errors: %s", strings.Join(errs, "; "))
	}

	s.logger.Info("webhook notifications sent", "event", msg.EventType, "count", len(s.cfg.WebhookURLs))
	return nil
}

func buildSlackFields(data map[string]any) []map[string]any {
	var fields []map[string]any
	for k, v := range data {
		fields = append(fields, map[string]any{
			"title": k,
			"value": fmt.Sprintf("%v", v),
			"short": true,
		})
	}
	return fields
}
