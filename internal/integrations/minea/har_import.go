package minea

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SessionFromHAR parses a Chrome DevTools HAR (1.2) and extracts a Minea session from the first
// successful (HTTP 200) POST to an AWS AppSync GraphQL URL. The Authorization header must carry
// the Cognito id token (raw JWT, no "Bearer " prefix — same as the browser).
func SessionFromHAR(raw []byte) (sessionFile, string, error) {
	var doc struct {
		Log struct {
			Entries []struct {
				Request struct {
					Method  string `json:"method"`
					URL     string `json:"url"`
					Headers []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"headers"`
				} `json:"request"`
				Response struct {
					Status int `json:"status"`
				} `json:"response"`
			} `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return sessionFile{}, "", fmt.Errorf("minea har: parse json: %w", err)
	}

	var (
		bestTok string
		bestURL string
		fallbackTok, fallbackURL string
	)

	for _, e := range doc.Log.Entries {
		u := e.Request.URL
		if !strings.Contains(u, "appsync-api.") || !strings.Contains(u, "/graphql") {
			continue
		}
		if !strings.EqualFold(e.Request.Method, httpMethodPost) {
			continue
		}
		auth := ""
		for _, h := range e.Request.Headers {
			if strings.EqualFold(h.Name, "authorization") && strings.TrimSpace(h.Value) != "" {
				auth = strings.TrimSpace(h.Value)
				break
			}
		}
		if auth == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			auth = strings.TrimSpace(auth[7:])
		}
		if e.Response.Status == httpStatusOK {
			bestTok, bestURL = auth, u
			break
		}
		if fallbackTok == "" {
			fallbackTok, fallbackURL = auth, u
		}
	}

	tok, u := bestTok, bestURL
	if tok == "" {
		tok, u = fallbackTok, fallbackURL
	}
	if tok == "" {
		return sessionFile{}, "", errors.New("minea har: no AppSync POST with Authorization found (export HAR while logged in; include a successful GraphQL call)")
	}

	userID, err := cognitoSubFromJWT(tok)
	if err != nil {
		return sessionFile{}, "", fmt.Errorf("minea har: jwt sub: %w", err)
	}

	sf := sessionFile{
		AuthToken: tok,
		UserID:    userID,
		SavedAt:   time.Now().UTC(),
	}
	return sf, u, nil
}

const httpMethodPost = "POST"
const httpStatusOK = 200

func cognitoSubFromJWT(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", errors.New("invalid jwt shape")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return "", err
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", errors.New("jwt missing sub")
	}
	return sub, nil
}

// ImportMineaHAR reads a HAR file and writes minea_session.json (same shape as Rod login).
// It returns the AppSync GraphQL URL found in the HAR; set MINEA_GRAPHQL_URL to that value (or rely on the new default).
func ImportMineaHAR(harPath, sessionPath string) (graphqlURL string, err error) {
	raw, err := os.ReadFile(harPath)
	if err != nil {
		return "", err
	}
	sf, gqlURL, err := SessionFromHAR(raw)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(sf)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(sessionPath, out, 0o600); err != nil {
		return "", err
	}
	return gqlURL, nil
}
