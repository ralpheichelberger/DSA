package notify

import (
	"context"

	"go.uber.org/zap"
)

type LogNotifier struct{ logger *zap.Logger }

func NewLogNotifier(logger *zap.Logger) *LogNotifier {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &LogNotifier{logger: logger}
}

func (l *LogNotifier) Name() string     { return "log" }
func (l *LogNotifier) Configured() bool { return true }

func (l *LogNotifier) Send(ctx context.Context, n Notification) error {
	_ = ctx
	fields := []zap.Field{
		zap.String("severity", n.Severity),
		zap.String("subject", n.Subject),
		zap.String("body", n.Body),
	}
	for k, v := range n.Context {
		fields = append(fields, zap.String(k, v))
	}

	switch SeverityLevel(n.Severity) {
	case 2:
		l.logger.Error("notification", fields...)
	case 1:
		l.logger.Warn("notification", fields...)
	default:
		l.logger.Info("notification", fields...)
	}
	return nil
}
