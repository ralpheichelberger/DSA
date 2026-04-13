package notify

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailNotifier_NotConfiguredWhenMissingFields(t *testing.T) {
	assert.False(t, NewEmailNotifier("", "localhost", 2525, "user", "pass").Configured())
	assert.False(t, NewEmailNotifier("a@b.com", "localhost", 2525, "", "pass").Configured())
}

func TestEmailNotifier_SubjectIncludesSeverity(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	done := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- ""
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		write := func(s string) {
			_, _ = writer.WriteString(s + "\r\n")
			_ = writer.Flush()
		}

		write("220 localhost ESMTP")
		var dataLines []string
		inData := false
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimRight(line, "\r\n")
			if inData {
				if line == "." {
					write("250 OK")
					inData = false
					continue
				}
				dataLines = append(dataLines, line)
				continue
			}

			upper := strings.ToUpper(line)
			switch {
			case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
				write("250-localhost")
				write("250 AUTH PLAIN")
			case strings.HasPrefix(upper, "AUTH"):
				write("235 2.7.0 Authentication successful")
			case strings.HasPrefix(upper, "MAIL FROM"):
				write("250 OK")
			case strings.HasPrefix(upper, "RCPT TO"):
				write("250 OK")
			case strings.HasPrefix(upper, "DATA"):
				write("354 End data with <CR><LF>.<CR><LF>")
				inData = true
			case strings.HasPrefix(upper, "QUIT"):
				write("221 Bye")
				done <- strings.Join(dataLines, "\n")
				return
			default:
				write("250 OK")
			}
		}
		done <- strings.Join(dataLines, "\n")
	}()

	addr := ln.Addr().(*net.TCPAddr)
	n := NewEmailNotifier("owner@example.com", "127.0.0.1", addr.Port, "user@example.com", "pass")
	err = n.Send(context.Background(), newTestNotification(SeverityCritical))
	require.NoError(t, err)

	select {
	case raw := <-done:
		assert.Contains(t, raw, "[CRITICAL]")
		assert.Contains(t, raw, "Credits below safe threshold")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for smtp capture")
	}
}
