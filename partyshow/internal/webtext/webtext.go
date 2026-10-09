// Package webtext fetches a public web page and extracts its readable text.
//
// The URL comes from customers, so the fetcher refuses to connect to private,
// loopback and link-local addresses (no requests into the server's own network).
package webtext

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"html"
)

// MaxBytes is the most HTML read from one page.
const MaxBytes = 2 << 20

// ErrBlockedAddress is returned when a URL resolves to a non-public address.
var ErrBlockedAddress = errors.New("address is not a public internet address")

// Fetcher downloads pages. AllowPrivate is only for tests.
type Fetcher struct {
	Client       *http.Client
	AllowPrivate bool
}

// New returns a Fetcher with safe timeouts and address checks.
func New() *Fetcher {
	f := &Fetcher{}
	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: f.control}
	f.Client = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			// No proxy: the address check must see the real destination, not a proxy.
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 8 * time.Second,
			MaxIdleConns:          10,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return checkScheme(req.URL)
		},
	}
	return f
}

// control runs on every outgoing connection after DNS resolution, so redirects
// and DNS tricks can't reach private addresses either.
func (f *Fetcher) control(_, address string, _ syscall.RawConn) error {
	if f.AllowPrivate {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !IsPublicIP(ip) {
		return ErrBlockedAddress
	}
	return nil
}

// IsPublicIP reports whether ip is a routable public address.
func IsPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	// Carrier-grade NAT 100.64.0.0/10 is not covered by IsPrivate.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1]&0xc0 == 64 {
		return false
	}
	return true
}

func checkScheme(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	return nil
}

// NormalizeURL adds https:// when the scheme is missing and validates the result.
func NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty URL")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if err := checkScheme(u); err != nil {
		return "", err
	}
	if u.Hostname() == "" {
		return "", errors.New("URL has no host")
	}
	return u.String(), nil
}

// Page is the readable content of a page.
type Page struct {
	Title       string
	Description string
	Text        string
}

// Fetch downloads rawURL and returns its text, cut to maxChars.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string, maxChars int) (Page, error) {
	u, err := NormalizeURL(rawURL)
	if err != nil {
		return Page{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Page{}, err
	}
	req.Header.Set("User-Agent", "PartyShowBot/1.0 (+quiz generator; fetches one page on request)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := f.Client.Do(req)
	if err != nil {
		return Page{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Page{}, fmt.Errorf("website answered %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes))
	if err != nil {
		return Page{}, err
	}
	p := Extract(string(body))
	p.Text = truncate(p.Text, maxChars)
	return p, nil
}

var (
	reDrop        = regexp.MustCompile(`(?is)<(script|style|noscript|svg|template|iframe|head)\b.*?</(script|style|noscript|svg|template|iframe|head)>`)
	reTitle       = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reDescription = regexp.MustCompile(`(?is)<meta[^>]+name=["']description["'][^>]*content=["']([^"']*)["']`)
	reBlock       = regexp.MustCompile(`(?i)</?(p|div|br|li|h[1-6]|section|article|tr|td|header|footer|nav|ul|ol)\b[^>]*>`)
	reTag         = regexp.MustCompile(`(?s)<[^>]*>`)
	reComment     = regexp.MustCompile(`(?s)<!--.*?-->`)
	reSpaces      = regexp.MustCompile(`[ \t\r\f\v]+`)
	reNewlines    = regexp.MustCompile(`\n\s*\n+`)
)

// Extract turns HTML into a title, a description and plain text.
func Extract(doc string) Page {
	var p Page
	if m := reTitle.FindStringSubmatch(doc); m != nil {
		p.Title = clean(html.UnescapeString(reTag.ReplaceAllString(m[1], "")))
	}
	if m := reDescription.FindStringSubmatch(doc); m != nil {
		p.Description = clean(html.UnescapeString(m[1]))
	}
	doc = reComment.ReplaceAllString(doc, " ")
	doc = reDrop.ReplaceAllString(doc, " ")
	doc = reBlock.ReplaceAllString(doc, "\n")
	doc = reTag.ReplaceAllString(doc, " ")
	doc = html.UnescapeString(doc)
	lines := strings.Split(doc, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if l = clean(l); l != "" {
			out = append(out, l)
		}
	}
	p.Text = reNewlines.ReplaceAllString(strings.Join(out, "\n"), "\n")
	return p
}

func clean(s string) string {
	return strings.TrimSpace(reSpaces.ReplaceAllString(s, " "))
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	s = s[:max]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
