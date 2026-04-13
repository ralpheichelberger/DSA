//go:build integration

package minea

import (
	"context"
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestScraper_RealLogin(t *testing.T) {
	email := os.Getenv("MINEA_EMAIL")
	pass := os.Getenv("MINEA_PASSWORD")
	if email == "" || pass == "" {
		t.Skip("MINEA_EMAIL and MINEA_PASSWORD are required")
	}

	s := NewScraper(email, pass, "./data/minea_session.json", zap.NewNop())
	if err := s.ensureAuth(context.Background()); err != nil {
		t.Fatalf("ensureAuth failed: %v", err)
	}
	if s.authToken == "" || s.userID == "" {
		t.Fatalf("auth token or user id not set")
	}
}

func TestScraper_RealGetCredits(t *testing.T) {
	email := os.Getenv("MINEA_EMAIL")
	pass := os.Getenv("MINEA_PASSWORD")
	if email == "" || pass == "" {
		t.Skip("MINEA_EMAIL and MINEA_PASSWORD are required")
	}
	s := NewScraper(email, pass, "./data/minea_session.json", zap.NewNop())
	if err := s.ensureAuth(context.Background()); err != nil {
		t.Fatalf("ensureAuth failed: %v", err)
	}
	b, err := s.GetCredits(context.Background())
	if err != nil {
		t.Fatalf("GetCredits failed: %v", err)
	}
	if b.Credits <= 0 {
		t.Fatalf("credits should be positive")
	}
}

func TestScraper_RealGetTrendingProducts(t *testing.T) {
	email := os.Getenv("MINEA_EMAIL")
	pass := os.Getenv("MINEA_PASSWORD")
	if email == "" || pass == "" {
		t.Skip("MINEA_EMAIL and MINEA_PASSWORD are required")
	}
	s := NewScraper(email, pass, "./data/minea_session.json", zap.NewNop())
	products, err := s.GetTrendingProducts(context.Background(), "", "US", 5)
	if err != nil {
		t.Fatalf("GetTrendingProducts failed: %v", err)
	}
	if len(products) < 1 {
		t.Fatalf("expected at least one product")
	}
}

func TestScraper_RealInsufficientCreditsSimulated(t *testing.T) {
	t.Skip("run only when account credits are below 20")
}
