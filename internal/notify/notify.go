package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dropshipagent/agent/config"
	"go.uber.org/zap"
)

const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

func SeverityLevel(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case SeverityWarning:
		return 1
	case SeverityCritical:
		return 2
	default:
		return 0
	}
}

type Notification struct {
	Severity  string
	Subject   string
	Body      string
	Context   map[string]string
	Timestamp time.Time
}

type Notifier interface {
	Send(ctx context.Context, n Notification) error
	Name() string
	Configured() bool
}

type MultiNotifier struct {
	notifiers   []Notifier
	minSeverity int
	logger      *zap.Logger
}

func New(cfg *config.Config, logger *zap.Logger) *MultiNotifier {
	if logger == nil {
		logger = zap.NewNop()
	}

	all := []Notifier{
		NewSlackNotifier(cfg.NotifySlackWebhook),
		NewDiscordNotifier(cfg.NotifyDiscordWebhook),
		NewTelegramNotifier(cfg.NotifyTelegramToken, cfg.NotifyTelegramChatID),
		NewEmailNotifier(cfg.NotifyEmail, cfg.NotifySMTPHost, cfg.NotifySMTPPort, cfg.NotifySMTPUser, cfg.NotifySMTPPassword),
	}

	configured := make([]Notifier, 0, len(all)+1)
	for _, n := range all {
		if n.Configured() {
			configured = append(configured, n)
		}
	}

	if len(configured) == 0 {
		logger.Warn("No notification channels configured — critical alerts will only appear in the dashboard")
	}

	configured = append(configured, NewLogNotifier(logger))
	return &MultiNotifier{
		notifiers:   configured,
		minSeverity: SeverityLevel(cfg.NotifyMinSeverity),
		logger:      logger,
	}
}

func (m *MultiNotifier) Send(ctx context.Context, n Notification) error {
	return m.sendInternal(ctx, n, false)
}

func (m *MultiNotifier) SendCritical(ctx context.Context, subject string, body string, ctx_ map[string]string) error {
	return m.sendInternal(ctx, Notification{
		Severity:  SeverityCritical,
		Subject:   subject,
		Body:      body,
		Context:   ctx_,
		Timestamp: time.Now().UTC(),
	}, true)
}

func (m *MultiNotifier) SendWarning(ctx context.Context, subject string, body string) error {
	return m.Send(ctx, Notification{
		Severity:  SeverityWarning,
		Subject:   subject,
		Body:      body,
		Timestamp: time.Now().UTC(),
	})
}

func (m *MultiNotifier) Configured() bool {
	for _, n := range m.notifiers {
		if n.Name() != "log" {
			return true
		}
	}
	return false
}

func (m *MultiNotifier) ChannelNames() []string {
	names := make([]string, 0, len(m.notifiers))
	for _, n := range m.notifiers {
		names = append(names, n.Name())
	}
	return names
}

func (m *MultiNotifier) sendInternal(ctx context.Context, n Notification, bypassMin bool) error {
	if n.Timestamp.IsZero() {
		n.Timestamp = time.Now().UTC()
	}
	if !bypassMin && SeverityLevel(n.Severity) < m.minSeverity {
		return nil
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errs := make(chan error, len(m.notifiers))
	successes := make(chan struct{}, len(m.notifiers))

	for _, notifier := range m.notifiers {
		wg.Add(1)
		go func(nt Notifier) {
			defer wg.Done()
			if err := nt.Send(ctxTimeout, n); err != nil {
				m.logger.Error("notifier send failed", zap.String("notifier", nt.Name()), zap.Error(err))
				errs <- fmt.Errorf("%s: %w", nt.Name(), err)
				return
			}
			successes <- struct{}{}
		}(notifier)
	}

	wg.Wait()
	close(errs)
	close(successes)

	if len(successes) > 0 {
		return nil
	}
	var all []error
	for err := range errs {
		all = append(all, err)
	}
	if len(all) == 0 {
		return errors.New("all notifiers failed")
	}
	return errors.Join(all...)
}
