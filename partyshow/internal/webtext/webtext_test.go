package webtext

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sample = `<!doctype html><html><head><title>Acme &amp; Co</title>
<meta name="description" content="Wir bauen Raketen.">
<script>var x = "<p>hidden</p>";</script><style>p{color:red}</style></head>
<body><nav>Home</nav><!-- comment --><h1>Über uns</h1><p>Gegründet   1999 in Wien.</p>
<ul><li>50 Mitarbeiter</li><li>3 Standorte</li></ul></body></html>`

func TestExtract(t *testing.T) {
	p := Extract(sample)
	assert.Equal(t, "Acme & Co", p.Title)
	assert.Equal(t, "Wir bauen Raketen.", p.Description)
	assert.Contains(t, p.Text, "Gegründet 1999 in Wien.")
	assert.Contains(t, p.Text, "50 Mitarbeiter")
	assert.NotContains(t, p.Text, "hidden")
	assert.NotContains(t, p.Text, "color")
	assert.NotContains(t, p.Text, "comment")
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		in, want string
		wantErr  bool
	}{
		{"acme.example", "https://acme.example", false},
		{" http://acme.example/about ", "http://acme.example/about", false},
		{"ftp://acme.example", "", true},
		{"file:///etc/passwd", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := NormalizeURL(tt.in)
		if tt.wantErr {
			assert.Error(t, err, tt.in)
			continue
		}
		require.NoError(t, err, tt.in)
		assert.Equal(t, tt.want, got)
	}
}

func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"8.8.8.8", true}, {"2a00:1450:4001::1", true},
		{"127.0.0.1", false}, {"10.1.2.3", false}, {"192.168.0.1", false}, {"172.16.5.4", false},
		{"169.254.169.254", false}, {"100.64.0.1", false}, {"0.0.0.0", false}, {"::1", false}, {"fe80::1", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, IsPublicIP(net.ParseIP(tt.ip)), tt.ip)
	}
}

func TestFetchBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()

	f := New()
	_, err := f.Fetch(context.Background(), srv.URL, 1000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), ErrBlockedAddress.Error())
}

func TestFetchAllowedAndTruncated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()

	f := New()
	f.AllowPrivate = true
	p, err := f.Fetch(context.Background(), srv.URL, 20)
	require.NoError(t, err)
	assert.Equal(t, "Acme & Co", p.Title)
	assert.LessOrEqual(t, len(p.Text), 20)
}

func TestFetchNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	f := New()
	f.AllowPrivate = true
	_, err := f.Fetch(context.Background(), srv.URL, 100)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "404"))
}
