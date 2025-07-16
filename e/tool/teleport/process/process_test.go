package process

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestProxyWithoutLicense(t *testing.T) {
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{})
	require.NoError(t, err)

	config := &servicecfg.Config{
		DataDir: t.TempDir(),
		Proxy: servicecfg.ProxyConfig{
			Enabled: true,
		},
		Auth: servicecfg.AuthConfig{
			Enabled:    false,
			Preference: authPreference,
		},
		Logger: logtest.With("test", t.Name()),
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

	config := &servicecfg.Config{
		DataDir: "", // Invalid data dir
		Auth: servicecfg.AuthConfig{
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
		Logger:   logtest.With("test", t.Name()),
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
	require.NoError(t, err)

	config := &servicecfg.Config{
		DataDir: t.TempDir(),
		Auth: servicecfg.AuthConfig{
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
		Logger:   logtest.With("test", t.Name()),
	}

	config.SetAuthServerAddress(*utils.MustParseAddr("tcp://127.0.0.1:8080"))

	_, err = NewTeleport(config)
	require.True(t, trace.IsAccessDenied(err))
}

// TestFallbackFeaturesFromLicense tests that a Cloud auth server
// without connection to the Cloud API and no features
// stored in the backend will read the features from the license,
// instead of failing.
func TestFallbackFeaturesFromLicense(t *testing.T) {
	cfg := servicecfg.MakeDefaultConfig()
	cfg.DataDir = makeTempDir(t)
	cfg.Auth.Enabled = true
	cfg.SetAuthServerAddress(utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"})
	cfg.Auth.StorageConfig = backend.Config{
		Type: lite.GetName(),
		Params: backend.Params{
			"path": t.TempDir(),
		},
	}
	cfg.Auth.ListenAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}
	cfg.Auth.LicenseFile = "testdata/license-cloud.pem"
	cfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(cfg.DataDir, defaults.BackendDir)}

	t.Setenv("TELEPORT_CLOUD_HOSTPORT", "invalid.localhost")

	_, err := NewTeleport(cfg)
	require.NoError(t, err)
	require.Equal(t, modules.BuildEnterprise, modules.GetModules().BuildType())
	features := modules.GetModules().Features()
	require.True(t, features.Cloud)
}

// makeTempDir makes a temp dir with a shorter name than t.TempDir() in order to
// avoid https://github.com/golang/go/issues/62614.
func makeTempDir(t *testing.T) string {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "teleport-test-")
	require.NoError(t, err, "os.MkdirTemp() failed")
	t.Cleanup(func() { os.RemoveAll(tempDir) })
	return tempDir
}
