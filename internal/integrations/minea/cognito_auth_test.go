package minea

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLoginViaCognitoPassword_RespectsSkip(t *testing.T) {
	t.Setenv("MINEA_SKIP_COGNITO", "true")
	s := NewScraper("a@b.c", "secret", t.TempDir()+"/session.json", zap.NewNop())
	err := s.loginViaCognitoPassword(context.Background())
	require.Error(t, err)
}
