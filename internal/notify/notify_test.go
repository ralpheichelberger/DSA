package notify

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dropshipagent/agent/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type spyNotifier struct {
	name       string
	called     atomic.Int32
	shouldFail bool
	delay      time.Duration
}

func (s *spyNotifier) Send(ctx context.Context, n Notification) error {
	_ = n
	s.called.Add(1)
	if s.delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.delay):
		}
	}
	if s.shouldFail {
		return errors.New("spy error")
	}
	return nil
}
func (s *spyNotifier) Name() string     { return s.name }
func (s *spyNotifier) Configured() bool { return true }

func newTestNotification(severity string) Notification {
	return Notification{
		Severity: severity,
		Subject:  "Minea credits low",
		Body:     "Credits below safe threshold",
		Context: map[string]string{
			"credits": "95",
			"region":  "EU",
		},
		Timestamp: time.Now().UTC(),
	}
}

func TestMultiNotifier_SkipsBelowMinSeverity(t *testing.T) {
	spy := &spyNotifier{name: "spy"}
	m := &MultiNotifier{notifiers: []Notifier{spy}, minSeverity: SeverityLevel(SeverityCritical), logger: zap.NewNop()}

	err := m.Send(context.Background(), newTestNotification(SeverityInfo))
	assert.NoError(t, err)
	assert.Equal(t, int32(0), spy.called.Load())
}

func TestMultiNotifier_SendsAtOrAboveMinSeverity(t *testing.T) {
	spy := &spyNotifier{name: "spy"}
	m := &MultiNotifier{notifiers: []Notifier{spy}, minSeverity: SeverityLevel(SeverityWarning), logger: zap.NewNop()}

	err := m.Send(context.Background(), newTestNotification(SeverityWarning))
	assert.NoError(t, err)
	assert.Equal(t, int32(1), spy.called.Load())
}

func TestMultiNotifier_SendCritical_BypassesMinSeverity(t *testing.T) {
	first := &spyNotifier{name: "one"}
	second := &spyNotifier{name: "two"}
	m := &MultiNotifier{notifiers: []Notifier{first, second}, minSeverity: SeverityLevel(SeverityCritical), logger: zap.NewNop()}

	err := m.SendCritical(context.Background(), "Critical alert", "Something failed", map[string]string{"k": "v"})
	assert.NoError(t, err)
	assert.Equal(t, int32(1), first.called.Load())
	assert.Equal(t, int32(1), second.called.Load())
}

func TestMultiNotifier_ContinuesIfOneNotifierFails(t *testing.T) {
	fail := &spyNotifier{name: "fail", shouldFail: true}
	ok := &spyNotifier{name: "ok"}
	m := &MultiNotifier{notifiers: []Notifier{fail, ok}, minSeverity: SeverityLevel(SeverityInfo), logger: zap.NewNop()}

	_ = m.Send(context.Background(), newTestNotification(SeverityCritical))
	assert.Equal(t, int32(1), ok.called.Load())
}

func TestMultiNotifier_ReturnsErrorIfAllFail(t *testing.T) {
	a := &spyNotifier{name: "a", shouldFail: true}
	b := &spyNotifier{name: "b", shouldFail: true}
	m := &MultiNotifier{notifiers: []Notifier{a, b}, minSeverity: SeverityLevel(SeverityInfo), logger: zap.NewNop()}

	err := m.Send(context.Background(), newTestNotification(SeverityCritical))
	assert.Error(t, err)
}

func TestMultiNotifier_ReturnsNilIfOneSucceeds(t *testing.T) {
	a := &spyNotifier{name: "a", shouldFail: true}
	b := &spyNotifier{name: "b"}
	m := &MultiNotifier{notifiers: []Notifier{a, b}, minSeverity: SeverityLevel(SeverityInfo), logger: zap.NewNop()}

	err := m.Send(context.Background(), newTestNotification(SeverityCritical))
	assert.NoError(t, err)
}

func TestMultiNotifier_NoChannelsConfigured_LogsWarning(t *testing.T) {
	cfg := &config.Config{NotifyMinSeverity: SeverityCritical}
	m := New(cfg, zap.NewNop())
	assert.False(t, m.Configured())
	assert.NoError(t, m.Send(context.Background(), newTestNotification(SeverityCritical)))
}

func TestMultiNotifier_CompletesWithinTimeout(t *testing.T) {
	slow := &spyNotifier{name: "slow", delay: 15 * time.Second}
	m := &MultiNotifier{notifiers: []Notifier{slow}, minSeverity: SeverityLevel(SeverityInfo), logger: zap.NewNop()}

	start := time.Now()
	err := m.Send(context.Background(), newTestNotification(SeverityCritical))
	elapsed := time.Since(start)
	assert.Error(t, err)
	assert.LessOrEqual(t, elapsed, 11*time.Second)
}

func TestSeverityLevel(t *testing.T) {
	testCases := []struct {
		in   string
		want int
	}{
		{SeverityInfo, 0},
		{SeverityWarning, 1},
		{SeverityCritical, 2},
		{"", 0},
		{"unknown", 0},
	}
	for _, tc := range testCases {
		assert.Equal(t, tc.want, SeverityLevel(tc.in))
	}
}

func TestLogNotifier_AlwaysConfigured(t *testing.T) {
	n := NewLogNotifier(zap.NewNop())
	assert.True(t, n.Configured())
	assert.NoError(t, n.Send(context.Background(), newTestNotification(SeverityInfo)))
	assert.NoError(t, n.Send(context.Background(), newTestNotification(SeverityWarning)))
	assert.NoError(t, n.Send(context.Background(), newTestNotification(SeverityCritical)))
}
