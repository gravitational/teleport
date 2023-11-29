package process

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestConfigureLicense(t *testing.T) {
	tt := []struct {
		name   string
		cfg    *servicecfg.Config
		assert func(*testing.T, *licensefile.LicenseFile, error)
	}{
		{
			name: "auth server without license should fail",
			cfg: &servicecfg.Config{
				Auth: servicecfg.AuthConfig{
					Enabled:     true,
					LicenseFile: "",
				},
			},
			assert: func(t *testing.T, lf *licensefile.LicenseFile, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "disabled auth server doesn't require a license",
			cfg: &servicecfg.Config{
				Auth: servicecfg.AuthConfig{
					Enabled: false,
				},
			},
			assert: func(t *testing.T, lf *licensefile.LicenseFile, err error) {
				require.NoError(t, err)
				require.Nil(t, lf) // returned licenseFile is nil in that case
			},
		},
		{
			name: "auth server with valid license should pass",
			cfg: &servicecfg.Config{
				Auth: servicecfg.AuthConfig{
					Enabled:     true,
					LicenseFile: "testdata/license-all-features.pem",
				},
				DataDir: t.TempDir(),
			},
			assert: func(t *testing.T, lf *licensefile.LicenseFile, err error) {
				require.NoError(t, err)
				require.NotNil(t, lf)
			},
		},
		{
			name: "auth server with missing license file should fail",
			cfg: &servicecfg.Config{
				Auth: servicecfg.AuthConfig{
					Enabled:     true,
					LicenseFile: "path/to/nonexistent/license/file",
				},
			},
			assert: func(t *testing.T, lf *licensefile.LicenseFile, err error) {
				require.Error(t, err)
				require.Nil(t, lf)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tc.cfg.Log = utils.NewLogger()
			tc.cfg.Logger = utils.NewSlogLoggerForTests()
			file, err := configureLicense(tc.cfg)
			tc.assert(t, file, err)
		})
	}
}
