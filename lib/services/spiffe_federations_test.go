// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package services

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestValidateSPIFFEFederation(t *testing.T) {
	t.Parallel()
	_ = time.Date(2000, 11, 2, 12, 0, 0, 0, time.UTC)

	var errContains = func(contains string) require.ErrorAssertionFunc {
		return func(t require.TestingT, err error, msgAndArgs ...interface{}) {
			require.ErrorContains(t, err, contains, msgAndArgs...)
		}
	}

	testCases := []struct {
		name       string
		in         *machineidv1.SPIFFEFederation
		requireErr require.ErrorAssertionFunc
	}{
		{
			name: "success - https_web",
			in: &machineidv1.SPIFFEFederation{
				Kind:    types.KindSPIFFEFederation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example.com",
				},
				Spec: &machineidv1.SPIFFEFederationSpec{
					BundleSource: &machineidv1.SPIFFEFederationBundleSource{
						HttpsWeb: &machineidv1.SPIFFEFederationBundleSourceHTTPSWeb{
							BundleEndpointUrl: "https://example.com/foo",
						},
					},
				},
			},
			requireErr: require.NoError,
		},
		{
			name: "success - static",
			in: &machineidv1.SPIFFEFederation{
				Kind:    types.KindSPIFFEFederation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example.com",
				},
				Spec: &machineidv1.SPIFFEFederationSpec{
					BundleSource: &machineidv1.SPIFFEFederationBundleSource{
						Static: &machineidv1.SPIFFEFederationBundleSourceStatic{
							Bundle: `{"keys":[{"use":"x509-svid","kty":"RSA","n":"1AgwZOvyaX_rdEzZsTk6WPAmW0rkz_yM2KTo_6tp8Qck7F1O75ssLUWRJh7IIZlWjXA0Nfc7DQiJw40ClGRds2kD-hJnsVa1UhP0QF9a02dP4ormhoCtOQMRsOJq4CkiuzowfkIRNkc1As5cMocAHhIKcu9H15fYEve390Oy7k3cJwTroRL0JXx8eYS32ae_d5S5QtgXYJvNpB1IumC2hJrkddTW97ozP53H6Vt6JdFpnZNqLXTCKm-pUebzEQ6RCCeLbKNS_NLvixL-4hlPelokUaMaPWnqZvJ0u4txhTSDbcwzjFXznqs6C9LUt3mzUQ_OudX1nsDk0wPab32HgQ","e":"AQAB","x5c":["MIIDnjCCAoagAwIBAgIQQwsx6y8q17q9cU6TsyuQ6zANBgkqhkiG9w0BAQsFADBpMRowGAYDVQQKExFsZWFmLnRlbGUub3R0ci5zaDEaMBgGA1UEAxMRbGVhZi50ZWxlLm90dHIuc2gxLzAtBgNVBAUTJjg5MTE2NDAzNDU0MzE5NjA1NDcyMDA1MDQyMDc3NDU1NTg1NTE1MB4XDTI0MDgwMTA5NDQ0NloXDTM0MDczMDA5NDQ0NlowaTEaMBgGA1UEChMRbGVhZi50ZWxlLm90dHIuc2gxGjAYBgNVBAMTEWxlYWYudGVsZS5vdHRyLnNoMS8wLQYDVQQFEyY4OTExNjQwMzQ1NDMxOTYwNTQ3MjAwNTA0MjA3NzQ1NTU4NTUxNTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBANQIMGTr8ml/63RM2bE5OljwJltK5M/8jNik6P+rafEHJOxdTu+bLC1FkSYeyCGZVo1wNDX3Ow0IicONApRkXbNpA/oSZ7FWtVIT9EBfWtNnT+KK5oaArTkDEbDiauApIrs6MH5CETZHNQLOXDKHAB4SCnLvR9eX2BL3t/dDsu5N3CcE66ES9CV8fHmEt9mnv3eUuULYF2CbzaQdSLpgtoSa5HXU1ve6Mz+dx+lbeiXRaZ2Tai10wipvqVHm8xEOkQgni2yjUvzS74sS/uIZT3paJFGjGj1p6mbydLuLcYU0g23MM4xV856rOgvS1Ld5s1EPzrnV9Z7A5NMD2m99h4ECAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgGmMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFFrGpt59RUgaAoD5MQEa8McxNOIPMA0GCSqGSIb3DQEBCwUAA4IBAQBRhYIeYU7YW+DxQgIB4gtoM+d0+FOCbteq2IAOjCzZC0rqds4nO1hCFSLaBa8a7GIrW4KoObBJDANRXUokrbbm+zwzj66Rcjj9cKOw/YTbYDu/FXGxY4nNwuI0oFg5CWV+XGepQ5xtGavISTqFK+ctrrIhlvhw+z4xMz4kw6IsMta+WrYPJv1DDoAm/qdZ8Ituvx1cx8THLCSxiXCVgm+AwzlKV4CXU12oY6xrBbHrSxUb/EfnX1KkvFGqQgjDZE+PjClNIN1qS0G2BuWYtjl3pRhiuA9I4B4u31F3wX7LtSlNdzesJeTEGJzcmgHjVr5voozHIeNq/fV/SgjYz2Do"]}],"spiffe_refresh_hint":300}`,
						},
					},
				},
			},
			requireErr: require.NoError,
		},
		{
			name:       "fail - null",
			in:         nil,
			requireErr: errContains("object cannot be nil"),
		},
		{
			name: "fail - nil metadata",
			in: &machineidv1.SPIFFEFederation{
				Kind:    types.KindSPIFFEFederation,
				Version: types.V1,
				Spec: &machineidv1.SPIFFEFederationSpec{
					BundleSource: &machineidv1.SPIFFEFederationBundleSource{
						HttpsWeb: &machineidv1.SPIFFEFederationBundleSourceHTTPSWeb{
							BundleEndpointUrl: "https://example.com/foo",
						},
					},
				},
			},
			requireErr: errContains("metadata: is required"),
		},
		{
			name: "fail - no name",
			in: &machineidv1.SPIFFEFederation{
				Kind:    types.KindSPIFFEFederation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "",
				},
				Spec: &machineidv1.SPIFFEFederationSpec{
					BundleSource: &machineidv1.SPIFFEFederationBundleSource{
						HttpsWeb: &machineidv1.SPIFFEFederationBundleSourceHTTPSWeb{
							BundleEndpointUrl: "https://example.com/foo",
						},
					},
				},
			},
			requireErr: errContains("metadata.name: is required"),
		},
		{
			name: "fail - bad url",
			in: &machineidv1.SPIFFEFederation{
				Kind:    types.KindSPIFFEFederation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example.com",
				},
				Spec: &machineidv1.SPIFFEFederationSpec{
					BundleSource: &machineidv1.SPIFFEFederationBundleSource{
						HttpsWeb: &machineidv1.SPIFFEFederationBundleSourceHTTPSWeb{
							BundleEndpointUrl: ":::::",
						},
					},
				},
			},
			requireErr: errContains("validating spec.bundle_source.https_web.bundle_endpoint_url"),
		},
		{
			name: "fail - bad bundle",
			in: &machineidv1.SPIFFEFederation{
				Kind:    types.KindSPIFFEFederation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example.com",
				},
				Spec: &machineidv1.SPIFFEFederationSpec{
					BundleSource: &machineidv1.SPIFFEFederationBundleSource{
						Static: &machineidv1.SPIFFEFederationBundleSourceStatic{
							Bundle: "xyzzy",
						},
					},
				},
			},
			requireErr: errContains("validating spec.bundle_source.static.bundle"),
		},
		{
			name: "fail - name contains prefix",
			in: &machineidv1.SPIFFEFederation{
				Kind:    types.KindSPIFFEFederation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "spiffe://example.com",
				},
				Spec: &machineidv1.SPIFFEFederationSpec{
					BundleSource: &machineidv1.SPIFFEFederationBundleSource{
						HttpsWeb: &machineidv1.SPIFFEFederationBundleSourceHTTPSWeb{
							BundleEndpointUrl: "https://example.com/foo",
						},
					},
				},
			},
			requireErr: errContains("metadata.name: must not include the spiffe:// prefix"),
		},
		{
			name: "fail - wrong kind",
			in: &machineidv1.SPIFFEFederation{
				Kind:    types.KindUser,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example.com",
				},
				Spec: &machineidv1.SPIFFEFederationSpec{
					BundleSource: &machineidv1.SPIFFEFederationBundleSource{
						HttpsWeb: &machineidv1.SPIFFEFederationBundleSourceHTTPSWeb{
							BundleEndpointUrl: "https://example.com/foo",
						},
					},
				},
			},
			requireErr: errContains(`kind: must be "spiffe_federation"`),
		},
		{
			name: "fail - wrong version",
			in: &machineidv1.SPIFFEFederation{
				Kind:    types.KindUser,
				Version: types.V3,
				Metadata: &headerv1.Metadata{
					Name: "example.com",
				},
				Spec: &machineidv1.SPIFFEFederationSpec{
					BundleSource: &machineidv1.SPIFFEFederationBundleSource{
						HttpsWeb: &machineidv1.SPIFFEFederationBundleSourceHTTPSWeb{
							BundleEndpointUrl: "https://example.com/foo",
						},
					},
				},
			},
			requireErr: errContains(`version: only "v1" is supported`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSPIFFEFederation(tc.in)
			tc.requireErr(t, err)
		})
	}
}

func TestSPIFFEFederationMarshaling(t *testing.T) {
	t.Parallel()

	testTime := time.Date(2000, 11, 2, 12, 0, 0, 0, time.UTC)
	testCases := []struct {
		name string
		in   *machineidv1.SPIFFEFederation
	}{
		{
			name: "normal",
			in: &machineidv1.SPIFFEFederation{
				Kind:    types.KindSPIFFEFederation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &machineidv1.SPIFFEFederationSpec{
					BundleSource: &machineidv1.SPIFFEFederationBundleSource{
						HttpsWeb: &machineidv1.SPIFFEFederationBundleSourceHTTPSWeb{
							BundleEndpointUrl: "https://example.com",
						},
					},
				},
				Status: &machineidv1.SPIFFEFederationStatus{
					CurrentBundle:         "xyzzy",
					CurrentBundleSyncedAt: timestamppb.New(testTime),
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotBytes, err := MarshalSPIFFEFederation(tc.in)
			require.NoError(t, err)
			// Test that unmarshaling gives us the same object
			got, err := UnmarshalSPIFFEFederation(gotBytes)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tc.in, got, protocmp.Transform()))
		})
	}
}
