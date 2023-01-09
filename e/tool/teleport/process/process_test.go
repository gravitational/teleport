package process

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
)

func TestProxyWithoutLicense(t *testing.T) {
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{})
	require.NoError(t, err)

	config := &service.Config{
		DataDir: t.TempDir(),
		Proxy: service.ProxyConfig{
			Enabled: true,
		},
		Auth: service.AuthConfig{
			Enabled:    false,
			Preference: authPreference,
		},
		Log: utils.WrapLogger(logrus.WithField("test", t.Name())),
	}

	config.SetAuthServerAddress(*utils.MustParseAddr("tcp://127.0.0.1:8080"))

	_, err = NewTeleport(config)
	require.NoError(t, err)
}

// TestModulesSetBeforeAuth tests that the global modules have been set with an
// enterprise license before the auth server is created. The creation of the
// auth server will still fail with a configuration error due to an empty
// DataDir, but that is sufficient for this test. Creating an auth server
// requires more extensive config that is not really needed here.
func TestModulesSetBeforeAuth(t *testing.T) {
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type: "local",
	})
	require.NoError(t, err)

	config := &service.Config{
		DataDir: "", // Invalid data dir
		Auth: service.AuthConfig{
			Enabled: true,
			StorageConfig: backend.Config{
				Type: lite.GetName(),
				Params: backend.Params{
					"path": t.TempDir(),
				},
			},
			StaticTokens: types.DefaultStaticTokens(),
			NoAudit:      true,
			Preference:   authPreference,
			ListenAddr:   *utils.MustParseAddr("tcp://127.0.0.1:0"),
			LicenseFile:  "testdata/license-all-features.pem",
		},
		Hostname: "localhost",
		Log:      utils.WrapLogger(logrus.WithField("test", t.Name())),
	}

	config.SetAuthServerAddress(*utils.MustParseAddr("tcp://127.0.0.1:8080"))

	_, err = NewTeleport(config)
	require.True(t, trace.IsBadParameter(err), "NewTeleport returned err = %T, want trace.BadParameterError", err)

	require.Equal(t, modules.BuildEnterprise, modules.GetModules().BuildType())
}

func TestMissingLicenseError(t *testing.T) {
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type: "local",
	})
	require.Nil(t, err)

	config := &service.Config{
		DataDir: t.TempDir(),
		Auth: service.AuthConfig{
			Enabled: true,
			StorageConfig: backend.Config{
				Type: lite.GetName(),
				Params: backend.Params{
					"path": t.TempDir(),
				},
			},
			StaticTokens: types.DefaultStaticTokens(),
			NoAudit:      true,
			Preference:   authPreference,
			ListenAddr:   *utils.MustParseAddr("tcp://127.0.0.1:0"),
		},
		Hostname: "localhost",
		Log:      utils.WrapLogger(logrus.WithField("test", t.Name())),
	}

	config.SetAuthServerAddress(*utils.MustParseAddr("tcp://127.0.0.1:8080"))

	_, err = NewTeleport(config)
	require.True(t, trace.IsAccessDenied(err))
}
