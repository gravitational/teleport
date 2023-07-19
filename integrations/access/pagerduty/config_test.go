package pagerduty

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func newMinimalValidConfig() Config {
	return Config{
		Pagerduty: PagerdutyConfig{
			APIKey:    "some-api-key-string",
			UserEmail: "root@localhost",
		},
	}
}

func TestInvalidConfigFailsCheck(t *testing.T) {

	t.Run("API Key", func(t *testing.T) {
		uut := newMinimalValidConfig()
		uut.Pagerduty.APIKey = ""
		require.Error(t, uut.CheckAndSetDefaults())
	})

	t.Run("User Email", func(t *testing.T) {
		uut := newMinimalValidConfig()
		uut.Pagerduty.UserEmail = ""
		require.Error(t, uut.CheckAndSetDefaults())
	})
}

func TestConfigDefaults(t *testing.T) {
	uut := Config{
		Pagerduty: PagerdutyConfig{
			APIKey:    "some-api-key-string",
			UserEmail: "root@localhost",
		},
	}
	require.NoError(t, uut.CheckAndSetDefaults())
	require.Equal(t, APIEndpointDefaultURL, uut.Pagerduty.APIEndpoint)
	require.Equal(t, NotifyServiceDefaultAnnotation, uut.Pagerduty.RequestAnnotations.NotifyService)
	require.Equal(t, ServicesDefaultAnnotation, uut.Pagerduty.RequestAnnotations.Services)
	require.Equal(t, "stderr", uut.Log.Output)
	require.Equal(t, "info", uut.Log.Severity)
}
