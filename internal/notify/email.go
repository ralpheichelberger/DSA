package notify

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

type EmailNotifier struct {
	toAddress string
	smtpHost  string
	smtpPort  int
	smtpUser  string
	smtpPass  string
}

func NewEmailNotifier(to, host string, port int, user, pass string) *EmailNotifier {
	return &EmailNotifier{
		toAddress: to,
		smtpHost:  host,
		smtpPort:  port,
		smtpUser:  user,
		smtpPass:  pass,
	}
}

func (e *EmailNotifier) Name() string { return "email" }
func (e *EmailNotifier) Configured() bool {
	return e.toAddress != "" && e.smtpUser != "" && e.smtpPass != ""
}

func (e *EmailNotifier) Send(ctx context.Context, n Notification) error {
	subject := fmt.Sprintf("[Dropship Agent][%s] %s", stringsUpper(n.Severity), n.Subject)
	var sb strings.Builder
	sb.WriteString(n.Body)
	sb.WriteString("\n\n")
	for k, v := range n.Context {
		sb.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}

	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", e.smtpUser, e.toAddress, subject, sb.String())
	addr := fmt.Sprintf("%s:%d", e.smtpHost, e.smtpPort)
	auth := smtp.PlainAuth("", e.smtpUser, e.smtpPass, e.smtpHost)

	errCh := make(chan error, 1)
	go func() {
		errCh <- smtp.SendMail(addr, auth, e.smtpUser, []string{e.toAddress}, []byte(message))
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	case <-time.After(8 * time.Second):
		return fmt.Errorf("email notifier timeout")
	}
}

func stringsUpper(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}
