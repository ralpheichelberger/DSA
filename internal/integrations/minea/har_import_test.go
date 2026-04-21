package minea

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeJWT returns a 3-segment JWT with JSON payload containing sub (for HAR fixture only).
func fakeJWT(sub string) string {
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	p := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"` + sub + `","exp":9999999999}`))
	return h + "." + p + ".x"
}

func TestSessionFromHAR_FindsAppSyncGraphQL(t *testing.T) {
	tok := fakeJWT("user-uuid-1")
	gqlURL := "https://abc123.appsync-api.eu-west-1.amazonaws.com/graphql"
	har := map[string]interface{}{
		"log": map[string]interface{}{
			"entries": []interface{}{
				map[string]interface{}{
					"request": map[string]interface{}{
						"method": "POST",
						"url":    gqlURL,
						"headers": []interface{}{
							map[string]interface{}{"name": "authorization", "value": tok},
						},
					},
					"response": map[string]interface{}{"status": 200},
				},
			},
		},
	}
	raw, err := json.Marshal(har)
	require.NoError(t, err)

	sf, foundURL, err := SessionFromHAR(raw)
	require.NoError(t, err)
	assert.Equal(t, gqlURL, foundURL)
	assert.Equal(t, tok, sf.AuthToken)
	assert.Equal(t, "user-uuid-1", sf.UserID)
	assert.False(t, sf.SavedAt.IsZero())
}

func TestSessionFromHAR_Prefers200Response(t *testing.T) {
	badTok := fakeJWT("bad")
	goodTok := fakeJWT("good-user")
	gqlURL := "https://x.appsync-api.eu-west-1.amazonaws.com/graphql"
	har := map[string]interface{}{
		"log": map[string]interface{}{
			"entries": []interface{}{
				map[string]interface{}{
					"request": map[string]interface{}{
						"method": "POST",
						"url":    gqlURL,
						"headers": []interface{}{
							map[string]interface{}{"name": "authorization", "value": badTok},
						},
					},
					"response": map[string]interface{}{"status": 401},
				},
				map[string]interface{}{
					"request": map[string]interface{}{
						"method": "POST",
						"url":    gqlURL,
						"headers": []interface{}{
							map[string]interface{}{"name": "Authorization", "value": goodTok},
						},
					},
					"response": map[string]interface{}{"status": 200},
				},
			},
		},
	}
	raw, _ := json.Marshal(har)
	sf, _, err := SessionFromHAR(raw)
	require.NoError(t, err)
	assert.Equal(t, goodTok, sf.AuthToken)
	assert.Equal(t, "good-user", sf.UserID)
}

func TestSessionFromHAR_ErrorsWhenMissing(t *testing.T) {
	har := map[string]interface{}{
		"log": map[string]interface{}{
			"entries": []interface{}{
				map[string]interface{}{
					"request": map[string]interface{}{
						"method": "GET",
						"url":    "https://app.minea.com/en",
						"headers": []interface{}{
							map[string]interface{}{"name": "accept", "value": "text/html"},
						},
					},
					"response": map[string]interface{}{"status": 200},
				},
			},
		},
	}
	raw, _ := json.Marshal(har)
	_, _, err := SessionFromHAR(raw)
	require.Error(t, err)
}

func TestWriteSessionFromHARFile(t *testing.T) {
	dir := t.TempDir()
	harPath := dir + "/x.har"
	outPath := dir + "/session.json"
	tok := fakeJWT("file-user")
	gqlURL := "https://z.appsync-api.eu-west-1.amazonaws.com/graphql"
	har := map[string]interface{}{
		"log": map[string]interface{}{
			"entries": []interface{}{
				map[string]interface{}{
					"request": map[string]interface{}{
						"method": "POST",
						"url":    gqlURL,
						"headers": []interface{}{
							map[string]interface{}{"name": "authorization", "value": tok},
						},
					},
					"response": map[string]interface{}{"status": 200},
				},
			},
		},
	}
	raw, err := json.Marshal(har)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(harPath, raw, 0o600))
	_, err = ImportMineaHAR(harPath, outPath)
	require.NoError(t, err)

	var sf sessionFile
	b, err := os.ReadFile(outPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &sf))
	assert.Equal(t, tok, sf.AuthToken)
	assert.Equal(t, "file-user", sf.UserID)
}
