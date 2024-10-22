package pro

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cloudflare/cfssl/csr"
	"github.com/google/uuid"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/license/constants"
	"github.com/gravitational/license/generate"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/e/lib/cloud/feature"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

func TestNewService(t *testing.T) {
	validLicenseFile, err := licensefile.FromPEM([]byte(newTestLicensePEM(t, map[entitlements.EntitlementKind]modules.EntitlementInfo{
		entitlements.AccessLists: modules.EntitlementInfo{Enabled: true},
	})))
	require.NoError(t, err)

	tt := []struct {
		name   string
		cfg    licenseUpdateServiceConfig
		assert func(*testing.T, *licenseUpdateService, error)
	}{
		{
			name: "minimum required fields",
			cfg: licenseUpdateServiceConfig{
				ServerID:    "ServerID",
				LicenseFile: validLicenseFile,
				LicensePath: "LicensePath",
			},
			assert: func(t *testing.T, s *licenseUpdateService, err error) {
				require.NoError(t, err)
				require.NotNil(t, s)
				require.Equal(t, "ServerID", s.serverID)
				require.Equal(t, "LicensePath", s.licensePath)
			},
		},
		{
			name: "overrides RequestTimeout and Interval with config values",
			cfg: licenseUpdateServiceConfig{
				ServerID:       "ServerID",
				LicenseFile:    validLicenseFile,
				LicensePath:    "LicensePath",
				RequestTimeout: time.Second,
				Interval:       time.Second,
			},
			assert: func(t *testing.T, s *licenseUpdateService, err error) {
				require.NoError(t, err)
				require.NotNil(t, s)
				require.Equal(t, time.Second, s.requestTimeout)
				require.Equal(t, time.Second, s.interval)
			},
		},
		{
			name: "ServerID is required",
			cfg: licenseUpdateServiceConfig{
				LicenseFile: validLicenseFile,
				LicensePath: "LicensePath",
			},
			assert: func(t *testing.T, s *licenseUpdateService, err error) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
			},
		},
		{
			name: "LicenseFile is required",
			cfg: licenseUpdateServiceConfig{
				ServerID:    "ServerID",
				LicensePath: "LicensePath",
			},
			assert: func(t *testing.T, s *licenseUpdateService, err error) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
			},
		},
		{
			name: "LicensePath is required",
			cfg: licenseUpdateServiceConfig{
				ServerID:    "ServerID",
				LicensePath: "LicensePath",
			},
			assert: func(t *testing.T, s *licenseUpdateService, err error) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s, err := newLicenseUpdateService(tc.cfg)
			tc.assert(t, s, err)
		})
	}
}

func TestWriteLicense(t *testing.T) {
	tt := []struct {
		name    string
		content []byte
		path    string
		assert  func(t *testing.T, err error, path string)
	}{
		{
			name: "successful write",
			path: func() string {
				tempDir := t.TempDir()
				licensePath := filepath.Join(tempDir, "license.pem")
				initialPem := []byte("initial license")
				require.NoError(t, os.WriteFile(licensePath, initialPem, 0644))
				return licensePath
			}(),
			content: []byte("new license"),
			assert: func(t *testing.T, err error, path string) {
				require.NoError(t, err)
				// assert file exists
				content, err := os.ReadFile(path)
				require.NoError(t, err)
				// assert content is updated
				require.Equal(t, []byte("new license"), content)
				// assert it preserves original file's permissions
				info, err := os.Stat(path)
				require.NoError(t, err)
				require.Equal(t, os.FileMode(0644), info.Mode().Perm(), "expected permissions to be preserved")
			},
		},
		{
			name:    "errors if license path does not exist instead of creating a new license",
			path:    filepath.Join(os.TempDir(), "nonexistent-dir", "license.pem"),
			content: []byte("new license"),
			assert: func(t *testing.T, err error, path string) {
				require.Error(t, err)
				require.ErrorIs(t, err, fs.ErrNotExist)
				// assert it doesnt create a file
				_, err = os.ReadFile(path)
				require.Error(t, err)
				require.ErrorIs(t, err, fs.ErrNotExist)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := writeLicense(tc.content, tc.path)
			tc.assert(t, err, tc.path)
		})
	}
}

func TestAppendAnonymizationKey(t *testing.T) {
	tt := []struct {
		name       string
		licensePEM []byte
		anonKey    []byte
		assert     func(t *testing.T, result []byte, err error)
	}{
		{
			name:       "successful append",
			licensePEM: []byte("-----BEGIN LICENSE-----\nlicense content\n-----END LICENSE-----\n"),
			anonKey:    []byte("anonymization key"),
			assert: func(t *testing.T, result []byte, err error) {
				require.NoError(t, err, "expected no error when appending anonymization key")
				// assert it starts with the original PEM
				require.True(t, bytes.HasPrefix(result, []byte("-----BEGIN LICENSE-----\nlicense content\n-----END LICENSE-----\n")))
				// assert it can be decoded
				decoded, _ := pem.Decode(result)
				require.NotNil(t, decoded, "expected resulting PEM to be valid")
				// assert it includes the anonymization key
				require.Equal(t, constants.AnonymizationKeyPEMBlock, decoded.Type, "expected PEM block type to match anonymization key")
				require.Equal(t, "anonymization key", string(decoded.Bytes))
			},
		},
		{
			name:       "empty anonKey",
			licensePEM: []byte("-----BEGIN LICENSE-----\nlicense content\n-----END LICENSE-----\n"),
			anonKey:    []byte{},
			assert: func(t *testing.T, result []byte, err error) {
				require.NoError(t, err, "expected no error when appending empty anonymization key")
				// assert it starts with the original PEM
				require.True(t, bytes.HasPrefix(result, []byte("-----BEGIN LICENSE-----\nlicense content\n-----END LICENSE-----\n")))
				// assert it can be decoded
				decoded, _ := pem.Decode(result)
				require.NotNil(t, decoded, "expected resulting PEM to be valid")
				// assert it includes the anonymization key
				require.Equal(t, "", string(decoded.Bytes))
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			result, err := appendAnonymizationKey(tc.licensePEM, tc.anonKey)
			tc.assert(t, result, err)
		})
	}
}

func TestLicenseUpdateService_Run(t *testing.T) {
	originalEntitlements := map[entitlements.EntitlementKind]modules.EntitlementInfo{
		entitlements.DB:  modules.EntitlementInfo{Enabled: true},
		entitlements.App: modules.EntitlementInfo{Enabled: true},
	}
	originalPEM := newTestLicensePEM(t, originalEntitlements)

	withAnonymizatonKey, err := generate.AppendAnonymizationKey([]byte(originalPEM))
	require.NoError(t, err)

	originalLicense, err := licensefile.FromPEM(withAnonymizatonKey)
	require.NoError(t, err)

	features := modules.Features{}
	features.Entitlements = originalEntitlements
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: features,
	})

	fakeClock := clockwork.NewFakeClock()

	// create the test license on disk
	licensePath := path.Join(t.TempDir(), "license.cert")
	err = os.WriteFile(licensePath, withAnonymizatonKey, 0644)
	require.NoError(t, err)

	client := new(testClient)
	// initially, client returns nothing
	client.setMockGetUpdatedLicense(func(ctx context.Context, r *cloudapi.GetUpdatedLicenseRequest) (*cloudapi.GetUpdatedLicenseResponse, error) {
		return &cloudapi.GetUpdatedLicenseResponse{}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service, err := newLicenseUpdateService(licenseUpdateServiceConfig{
		ServerID:    "ServerID",
		LicenseFile: originalLicense,
		LicensePath: licensePath,
		Interval:    time.Second,
		Clock:       fakeClock,
		NewClientFromTLSConfig: func(c *tls.Config) (cloud.Client, error) {
			return client, nil
		},
	})
	require.NoError(t, err)
	require.NotNil(t, service)

	go service.Run(ctx)
	fakeClock.BlockUntil(1)

	require.Equal(t, features, modules.GetModules().Features())

	// update client to return a new license
	newEntitlements := map[entitlements.EntitlementKind]modules.EntitlementInfo{
		entitlements.DB:  modules.EntitlementInfo{Enabled: false},
		entitlements.App: modules.EntitlementInfo{Enabled: false},
	}
	newPEM := newTestLicensePEM(t, newEntitlements)
	newLicense, err := licensefile.FromPEM([]byte(newPEM))
	// The new license does not include an anonymization key because the server does not have one.
	require.NoError(t, err)
	newLicenseEntitlements := feature.GetLicenseEntitlements(newLicense.License.GetEntitlements())

	client.setMockGetUpdatedLicense(func(ctx context.Context, r *cloudapi.GetUpdatedLicenseRequest) (*cloudapi.GetUpdatedLicenseResponse, error) {
		return &cloudapi.GetUpdatedLicenseResponse{
			Pem: newPEM,
		}, nil
	})

	// assert that the entitlements, licensefile, anonymization key and disk license eventually match the new license
	requireNewLicense(t, fakeClock, newLicenseEntitlements, originalLicense.License.GetAnonymizationKey(), 0644, newLicense, service.licensePath)

	// test that the service won't crash if it receives an error
	client.setMockGetUpdatedLicense(
		func(ctx context.Context, r *cloudapi.GetUpdatedLicenseRequest) (*cloudapi.GetUpdatedLicenseResponse, error) {
			return nil, errors.New("err fetching features")
		},
	)
	// require the same license as before
	requireNewLicense(t, fakeClock, newLicenseEntitlements, originalLicense.License.GetAnonymizationKey(), 0644, newLicense, service.licensePath)

	// assert that the service is still running and able to download a new license after getting an error from the server
	newEntitlements = map[entitlements.EntitlementKind]modules.EntitlementInfo{
		entitlements.DB:  modules.EntitlementInfo{Enabled: false},
		entitlements.App: modules.EntitlementInfo{Enabled: true},
		entitlements.K8s: modules.EntitlementInfo{Enabled: true},
	}
	newPEM = newTestLicensePEM(t, newEntitlements)
	newLicense, err = licensefile.FromPEM([]byte(newPEM))
	require.NoError(t, err)
	newLicenseEntitlements = feature.GetLicenseEntitlements(newLicense.License.GetEntitlements())
	client.setMockGetUpdatedLicense(
		func(ctx context.Context, r *cloudapi.GetUpdatedLicenseRequest) (*cloudapi.GetUpdatedLicenseResponse, error) {
			return &cloudapi.GetUpdatedLicenseResponse{
				Pem: newPEM,
			}, nil
		},
	)
	requireNewLicense(t, fakeClock, newLicenseEntitlements, originalLicense.License.GetAnonymizationKey(), 0644, newLicense, service.licensePath)
}

// requireNewLicense is a helper function that advances the clock and checks that
// the license is eventually the expected value
func requireNewLicense(t *testing.T,
	fakeClock clockwork.FakeClock,
	expectedEntitlements map[entitlements.EntitlementKind]modules.EntitlementInfo,
	expectedAnonKey string,
	expectedPerms uint32,
	newLicense *licensefile.LicenseFile,
	licensePath string,
) {
	t.Helper()
	fakeClock.Advance(time.Second * 2)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// assert that features got updated and match the new license
		assert.Equal(t, expectedEntitlements, modules.GetModules().Features().Entitlements)
		// assert that licensePath exists and matches the license
		diskContent, err := os.ReadFile(licensePath)
		assert.NoError(t, err)
		// the expected result is the new license full PEM encoded cert plus the anonymization block
		expected := append(newLicense.GetKeyPair().CertPEM, newLicense.GetKeyPair().KeyPEM...)
		expected, err = appendAnonymizationKey(expected, []byte(expectedAnonKey))
		assert.NoError(t, err)
		assert.Equal(t, expected, diskContent)
		// assert that the permissions matches the expected value
		info, err := os.Stat(licensePath)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, os.FileMode(expectedPerms), info.Mode().Perm())
	}, time.Second*10, time.Millisecond*100)
}

// testClient is a struct that implements cloud.Client with a
// mocked GetUpdatedLicense.
type testClient struct {
	cloud.MockedClient
	mu                    sync.Mutex
	mockGetUpdatedLicense func(ctx context.Context, r *cloudapi.GetUpdatedLicenseRequest) (*cloudapi.GetUpdatedLicenseResponse, error)
}

func (t *testClient) GetUpdatedLicense(ctx context.Context, r *cloudapi.GetUpdatedLicenseRequest, opts ...grpc.CallOption) (*cloudapi.GetUpdatedLicenseResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.mockGetUpdatedLicense != nil {
		return t.mockGetUpdatedLicense(ctx, r)
	}

	return nil, trace.NotImplemented("MockGetUpdatedLicense is not implemented")
}

func (t *testClient) setMockGetUpdatedLicense(f func(ctx context.Context, r *cloudapi.GetUpdatedLicenseRequest) (*cloudapi.GetUpdatedLicenseResponse, error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.mockGetUpdatedLicense = f
}

// Implement cloud.Client interface for mocked client
func (t *testClient) Close() error { return nil }

func newTestLicensePEM(t *testing.T, entitlements map[entitlements.EntitlementKind]modules.EntitlementInfo) string {
	ca, err := authority.GenerateSelfSignedCA(csr.CertificateRequest{
		CN:    "localhost",
		Names: csr.New().Names,
		Hosts: []string{"localhost"},
	})
	require.NoError(t, err)

	pk, err := generate.NewPrivateKey()
	require.NoError(t, err)

	// payload
	license := licensefile.LicenseFile{}
	license.License, err = types.NewLicense(uuid.NewString(), types.LicenseSpecV3{
		AccountID:    uuid.NewString(),
		Entitlements: entitlementKindMapToStringMap(entitlements),
	})
	require.NoError(t, err)

	license.License.SetExpiry(time.Now().UTC().Add(time.Hour))

	payload, err := json.Marshal(license.License)
	require.NoError(t, err)

	l, err := generate.NewLicense(generate.NewLicenseInfo{
		ValidFor:   time.Hour,
		PrivateKey: pk,
		TLSKeyPair: *ca,
		Payload:    payload,
	})
	require.NoError(t, err)
	return l
}

// entitlementKindMapToStringMap converts map[entitlements.EntitlementKind]modules.EntitlementInfo into the type expected
// in the license spec (map[string]types.EntitlementInfo).
func entitlementKindMapToStringMap(input map[entitlements.EntitlementKind]modules.EntitlementInfo) map[string]types.EntitlementInfo {
	output := make(map[string]types.EntitlementInfo)
	for key, value := range input {
		output[string(key)] = types.EntitlementInfo{
			Enabled: types.Bool(value.Enabled),
			Limit:   value.Limit,
		}
	}
	return output
}
