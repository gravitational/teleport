package process

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/gravitational/license"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
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
		{
			name: "auth server with a license valid for more than 100 years and self-hosted should fail",
			cfg: &servicecfg.Config{
				Auth: servicecfg.AuthConfig{
					Enabled:     true,
					LicenseFile: "testdata/license-deprecated.pem",
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
			tc.cfg.Logger = logtest.NewLogger()
			file, err := configureLicense(tc.cfg)
			tc.assert(t, file, err)
		})
	}
}

func TestIsLicenseDeprecated(t *testing.T) {
	tt := []struct {
		name     string
		license  *licensefile.LicenseFile
		expected bool
	}{
		{
			name: "non-cloud 100 year license generated in 2023 is deprecated",
			license: &licensefile.LicenseFile{
				KeyPair: &license.License{
					Cert: &x509.Certificate{
						NotBefore: time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
						NotAfter:  time.Now().AddDate(100, 0, 0),
					},
				},
			},
			expected: true,
		},
		{
			name: "cloud 100 year license generated in 2023 is not deprecated",
			license: &licensefile.LicenseFile{
				KeyPair: &license.License{
					Cert: &x509.Certificate{
						NotBefore: time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
						NotAfter:  time.Now().AddDate(100, 0, 0),
					},
				},
				License: &types.LicenseV3{
					Spec: types.LicenseSpecV3{
						Cloud: true,
					},
				},
			},
			expected: false,
		},
		{
			name: "non-cloud licenses valid for less than 4 year is not deprecated",
			license: &licensefile.LicenseFile{
				KeyPair: &license.License{
					Cert: &x509.Certificate{
						NotBefore: time.Now(),
						NotAfter:  time.Now().AddDate(3, 11, 15),
					},
				},
				License: &types.LicenseV3{
					Spec: types.LicenseSpecV3{
						Cloud: false,
					},
				},
			},
			expected: false,
		},
		{
			name: "licenses generated before 2024 valid for less than 4 years are not deprecated",
			license: &licensefile.LicenseFile{
				KeyPair: &license.License{
					Cert: &x509.Certificate{
						NotBefore: time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
						NotAfter:  time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC).AddDate(3, 11, 15),
					},
				},
				License: &types.LicenseV3{},
			},
			expected: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, isLicenseDeprecated(tc.license))
		})
	}
}
