package minea

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

// Defaults from the Minea web app (public Cognito app client id, eu-west-1 pool).
// Override with MINEA_COGNITO_REGION / MINEA_COGNITO_CLIENT_ID if Minea rotates them.
const (
	defaultCognitoRegion   = "eu-west-1"
	defaultCognitoClientID = "11idn2ot5orv6dm4f68lkrfu7d"
)

// cognitoPasswordAuth exchanges email/password for Cognito tokens using USER_PASSWORD_AUTH.
// No AWS account credentials are required; this is the same unauthenticated call the browser makes.
// If the app client disallows this flow (SRP-only), AWS returns an error and the caller should fall back to Rod.
func cognitoPasswordAuth(ctx context.Context, region, clientID, username, password string) (idToken, accessToken, refreshToken string, err error) {
	region = strings.TrimSpace(region)
	if region == "" {
		region = defaultCognitoRegion
	}
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		clientID = defaultCognitoClientID
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", "", "", fmt.Errorf("aws load config: %w", err)
	}
	client := cognitoidentityprovider.NewFromConfig(cfg)
	out, err := client.InitiateAuth(ctx, &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: types.AuthFlowTypeUserPasswordAuth,
		ClientId: aws.String(clientID),
		AuthParameters: map[string]string{
			"USERNAME": strings.TrimSpace(username),
			"PASSWORD": password,
		},
	})
	if err != nil {
		return "", "", "", err
	}
	if out.AuthenticationResult != nil {
		ar := out.AuthenticationResult
		return aws.ToString(ar.IdToken), aws.ToString(ar.AccessToken), aws.ToString(ar.RefreshToken), nil
	}
	if out.ChallengeName != "" {
		return "", "", "", fmt.Errorf("cognito challenge %q (complete in browser or extend the client)", out.ChallengeName)
	}
	return "", "", "", fmt.Errorf("cognito: empty authentication result")
}
