package process

import (
	"os"
	"path/filepath"
	"testing"

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestProxyWithoutLicense(t *testing.T) {
	config := servicecfg.MakeDefaultConfig()
	config.DataDir = t.TempDir()
	config.Auth.Enabled = false
	config.Auth.Preference = types.DefaultAuthPreference()
	config.SSH.Enabled = false

	config.SetAuthServerAddress(*utils.MustParseAddr("tcp://127.0.0.1:8080"))

	_, err := NewTeleport(config)
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

func TestNewAnonimizer(t *testing.T) {
	tests := []struct {
		name        string
		testModules modulestest.Modules
		license     *licensefile.LicenseFile
		wantKey     string
	}{
		{
			name: "uses CloudAnonymizationKey when present",
			testModules: modulestest.Modules{
				TestFeatures: modules.Features{CloudAnonymizationKey: []byte("cloud-key")},
			},
			license: &licensefile.LicenseFile{
				KeyPair: &liblicense.License{AnonymizationKey: []byte("license-key")},
			},
			wantKey: "cloud-key",
		},
		{
			name:        "uses license AnonymizationKey when no cloud key is present",
			testModules: modulestest.Modules{},
			license: &licensefile.LicenseFile{
				KeyPair: &liblicense.License{AnonymizationKey: []byte("license-key")},
			},
			wantKey: "license-key",
		},
		{
			name:        "uses cluster ID when neither cloud key nor license key is present",
			testModules: modulestest.Modules{},
			license:     &licensefile.LicenseFile{KeyPair: &liblicense.License{}},
			wantKey:     "cluster-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
				Dir:       t.TempDir(),
				ClusterID: "cluster-id",
			})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, testAuthServer.Close()) })

			modulestest.SetTestModules(t, tt.testModules)

			anonymizer, err := newAnonimizer(testAuthServer.AuthServer, tt.license)
			require.NoError(t, err)
			require.NotNil(t, anonymizer)

			require.Equal(t, tt.wantKey, string(testAuthServer.AuthServer.GetAnonymizationKey()))

			realAuthAnonymizer, err := utils.NewHMACAnonymizer(testAuthServer.AuthServer)
			require.NoError(t, err)

			staticAuthAnonymizer, err := utils.NewHMACAnonymizer(utils.AnonymizationKeyString(tt.wantKey))
			require.NoError(t, err)

			// The real anonymizer should produce the same output as the static one with the expected key.
			input := []byte("test-input")
			realOutput := realAuthAnonymizer.Anonymize(input)
			staticOutput := staticAuthAnonymizer.Anonymize(input)
			require.Equal(t, staticOutput, realOutput)

			// change the anonymization key and verify that the output changes accordingly
			testAuthServer.AuthServer.SetAnonymizationKey([]byte("otherKey"))

			staticAuthAnonymizer, err = utils.NewHMACAnonymizer(utils.AnonymizationKeyString("otherKey"))
			require.NoError(t, err)

			realOutput2 := realAuthAnonymizer.Anonymize(input)
			staticOutput2 := staticAuthAnonymizer.Anonymize(input)
			require.Equal(t, staticOutput2, realOutput2)
		})
	}
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
