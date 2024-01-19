package accessgraph

import (
	"context"
	"errors"
	"testing"

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

type mockAuth struct {
	ca          types.CertAuthority
	clusterName types.ClusterName
}

func (m *mockAuth) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	return m.ca, nil
}

func (m *mockAuth) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	return m.clusterName, nil
}

type mockRegistrator struct {
	register         func(config ServiceClientConfig, creds ClientCredentials, hostCAPem []byte, clusterName string) error
	registerCalled   int
	replaceCAs       func(config ServiceClientConfig, creds ClientCredentials, caPEMs [][]byte) error
	replaceCAsCalled int
}

func (m *mockRegistrator) Register(ctx context.Context, config ServiceClientConfig, creds ClientCredentials, hostCAPem []byte, clusterName string) error {
	m.registerCalled++
	if m.register != nil {
		return m.register(config, creds, hostCAPem, clusterName)
	}
	return nil
}

func (m *mockRegistrator) ReplaceCAs(ctx context.Context, config ServiceClientConfig, creds ClientCredentials, caPEMs [][]byte) error {
	m.replaceCAsCalled++
	if m.replaceCAs != nil {
		return m.replaceCAs(config, creds, caPEMs)
	}
	return nil
}

var (
	testConfig = ServiceClientConfig{
		Addr: "foo:123",
		CA:   "/foo/bar.pem",
	}
	testAdminCreds = ClientCredentials{
		CertPEM: []byte("admin cert"),
		KeyPEM:  []byte("admin key"),
	}
	testActiveKeyPair = &types.TLSKeyPair{
		Cert: []byte("active cert"),
		Key:  []byte("active key"),
	}
)

func createCA(t *testing.T, rotationPhase string, additionalKeyPair *types.TLSKeyPair) types.CertAuthority {
	spec := types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "localhost",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{testActiveKeyPair},
		},
	}
	if rotationPhase != "" {
		spec.Rotation = &types.Rotation{
			Phase: rotationPhase,
		}
	}
	if additionalKeyPair != nil {
		spec.AdditionalTrustedKeys = types.CAKeySet{
			TLS: []*types.TLSKeyPair{additionalKeyPair},
		}
	}
	ca, err := types.NewCertAuthority(spec)
	require.NoError(t, err)
	return ca
}

func createClusterName(t *testing.T) types.ClusterName {
	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "localhost",
	})
	require.NoError(t, err)
	return clusterName
}

func TestRegister_CallsReplaceCAsInAllCases(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := logrus.NewEntry(logrus.StandardLogger())

	ca := createCA(t, "", nil)
	clusterName := createClusterName(t)
	auth := &mockAuth{
		ca:          ca,
		clusterName: clusterName,
	}

	testCases := []struct {
		name            string
		registerError   error
		replaceCAsError error
	}{
		{
			name: "Register() success",
		},
		{
			name:          "Register() failure",
			registerError: errors.New("something happened"),
		},
		{
			name:            "Register() failure and ReplaceCAs() failure",
			registerError:   errors.New("something happened"),
			replaceCAsError: errors.New("something critical happened"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			registrator := &mockRegistrator{
				register: func(config ServiceClientConfig, creds ClientCredentials, hostCAPem []byte, name string) error {
					require.Equal(t, testConfig, config)
					require.Equal(t, testAdminCreds, creds)
					require.Equal(t, ca.GetActiveKeys().TLS[0].Cert, hostCAPem)
					require.Equal(t, clusterName.GetClusterName(), name)
					return tc.registerError
				},
				replaceCAs: func(config ServiceClientConfig, creds ClientCredentials, caPEMs [][]byte) error {
					require.Equal(t, testConfig, config)
					require.Equal(t, testAdminCreds, creds)
					require.Equal(t, [][]byte{ca.GetActiveKeys().TLS[0].Cert}, caPEMs)
					return tc.replaceCAsError
				},
			}
			err := Register(ctx, log, registrator, testConfig, testAdminCreds, auth, nil)
			require.Equal(t, 1, registrator.registerCalled)
			require.Equal(t, 1, registrator.replaceCAsCalled)

			if tc.registerError != nil && tc.replaceCAsError != nil {
				require.Error(t, err)
				unwrapped := trace.Unwrap(err)
				aerr, ok := unwrapped.(trace.Aggregate)
				require.True(t, ok, "expected error to be trace.Aggregate, was %T instead", unwrapped)
				// both errors should be reported
				require.Len(t, aerr.Errors(), 2)
				require.Equal(t, aerr.Errors()[0], tc.registerError)
				require.Equal(t, aerr.Errors()[1], tc.replaceCAsError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRegister_Cloud_UsesLicenseIdentity(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})
	ctx := context.Background()
	log := logrus.NewEntry(logrus.StandardLogger())

	ca := createCA(t, "", nil)
	clusterName := createClusterName(t)
	auth := &mockAuth{
		ca:          ca,
		clusterName: clusterName,
	}
	license := &licensefile.LicenseFile{
		KeyPair: &liblicense.License{
			CertPEM: []byte("license_cert"),
			KeyPEM:  []byte("license_key"),
		},
	}

	registrator := &mockRegistrator{
		register: func(config ServiceClientConfig, creds ClientCredentials, hostCAPem []byte, name string) error {
			require.Equal(t, testConfig, config)
			require.Equal(t, license.KeyPair.CertPEM, creds.CertPEM)
			require.Equal(t, license.KeyPair.KeyPEM, creds.KeyPEM)
			return nil
		},
		replaceCAs: func(config ServiceClientConfig, creds ClientCredentials, caPEMs [][]byte) error {
			require.Equal(t, testConfig, config)
			require.Equal(t, testAdminCreds, creds)
			return nil
		},
	}
	err := Register(ctx, log, registrator, testConfig, testAdminCreds, auth, license)
	require.NoError(t, err)
	require.Equal(t, 1, registrator.registerCalled)
	require.Equal(t, 1, registrator.replaceCAsCalled)
}

func TestRegister_CARotation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := logrus.NewEntry(logrus.StandardLogger())

	additionalKeyPair := &types.TLSKeyPair{
		Cert: []byte("additional cert"),
		Key:  []byte("additional key"),
	}

	testCases := []struct {
		rotationPhase      string
		additionalKeyPair  *types.TLSKeyPair
		expectedRegisterCA []byte
		expectedCAs        [][]byte
	}{
		{
			rotationPhase:      types.RotationPhaseStandby,
			expectedRegisterCA: testActiveKeyPair.Cert,
			expectedCAs:        [][]byte{testActiveKeyPair.Cert},
		},
		{
			rotationPhase:      types.RotationPhaseInit,
			additionalKeyPair:  additionalKeyPair,
			expectedRegisterCA: testActiveKeyPair.Cert,
			expectedCAs:        [][]byte{testActiveKeyPair.Cert},
		},
		{
			// As of this phase, CAs have swapped.
			// Expect to register with the additional ("old") CA, but add both old and new CAs as trusted.
			rotationPhase:      types.RotationPhaseUpdateClients,
			additionalKeyPair:  additionalKeyPair,
			expectedRegisterCA: additionalKeyPair.Cert,
			expectedCAs:        [][]byte{testActiveKeyPair.Cert, additionalKeyPair.Cert},
		},
		{
			rotationPhase:      types.RotationPhaseRollback,
			expectedRegisterCA: testActiveKeyPair.Cert,
			expectedCAs:        [][]byte{testActiveKeyPair.Cert},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.rotationPhase, func(t *testing.T) {
			ca := createCA(t, tc.rotationPhase, tc.additionalKeyPair)
			clusterName := createClusterName(t)
			auth := &mockAuth{
				ca:          ca,
				clusterName: clusterName,
			}

			registrator := &mockRegistrator{
				register: func(config ServiceClientConfig, creds ClientCredentials, hostCAPem []byte, name string) error {
					require.Equal(t, testConfig, config)
					require.Equal(t, testAdminCreds, creds)
					require.Equal(t, tc.expectedRegisterCA, hostCAPem)
					return nil
				},
				replaceCAs: func(config ServiceClientConfig, creds ClientCredentials, caPEMs [][]byte) error {
					require.Equal(t, testConfig, config)
					require.Equal(t, testAdminCreds, creds)
					require.Equal(t, tc.expectedCAs, caPEMs)
					return nil
				},
			}
			err := Register(ctx, log, registrator, testConfig, testAdminCreds, auth, nil)
			require.NoError(t, err)
			require.Equal(t, 1, registrator.registerCalled)
			require.Equal(t, 1, registrator.replaceCAsCalled)
		})
	}
}
