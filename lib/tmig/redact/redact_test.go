package redact

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactTokenSecret(t *testing.T) {
	input := `auth_token: "abc123secret456"`
	result := Config(input)
	require.NotContains(t, result, "abc123secret456")
	require.Contains(t, result, "<redacted>")
}

func TestRedactJoinParamsTokenName(t *testing.T) {
	input := `join_params:
  method: token
  token_name: my-secret-token-name`
	result := Config(input)
	// token_name is NOT a secret (it's a reference), should remain
	require.Contains(t, result, "my-secret-token-name")
}

func TestRedactAuthToken(t *testing.T) {
	input := `auth_token: supersecret123`
	result := Config(input)
	require.NotContains(t, result, "supersecret123")
	require.Contains(t, result, "<redacted>")
}

func TestRedactSecretsGeneric(t *testing.T) {
	input := "registration_secret: one-time-value-here"
	result := Secrets(input)
	require.NotContains(t, result, "one-time-value-here")
}

func TestRedactPreservesNonSecrets(t *testing.T) {
	input := `proxy_server: scoped.example.com:443
data_dir: /var/lib/teleport_dgxc-team-a`
	result := Config(input)
	require.Equal(t, input, result)
}

func TestRedactFixtureSecretsNeverSurvive(t *testing.T) {
	// Regression test: known fixture secrets must never appear in output
	fixtures := []string{
		"abc123secret456",
		"supersecret123",
		"one-time-value-here",
		"bearer-token-xyz",
	}
	input := `auth_token: abc123secret456
registration_secret: one-time-value-here
some_field: bearer-token-xyz`
	result := Secrets(input)
	for _, secret := range fixtures {
		require.NotContains(t, result, secret, "fixture secret %q leaked", secret)
	}
}
