package accessgraph

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/fixtures"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
)

type mockAuth struct {
	ca          types.CertAuthority
	clusterName types.ClusterName
}

func (m *mockAuth) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	return m.ca, nil
}

func (m *mockAuth) GetClusterName(_ context.Context) (types.ClusterName, error) {
	return m.clusterName, nil
}

type mockRegistrator struct {
	register         func(config ServiceClientConfig, getCreds ClientCredentialsGetter, hostCAPem []byte, clusterName string) error
	registerCalled   int
	replaceCAs       func(config ServiceClientConfig, getCreds ClientCredentialsGetter, caPEMs [][]byte) error
	replaceCAsCalled int
}

func (m *mockRegistrator) Register(ctx context.Context, config ServiceClientConfig, getCreds ClientCredentialsGetter, hostCAPem []byte, clusterName string) error {
	m.registerCalled++
	if m.register != nil {
		return m.register(config, getCreds, hostCAPem, clusterName)
	}
	return nil
}

func (m *mockRegistrator) ReplaceCAs(ctx context.Context, config ServiceClientConfig, getCreds ClientCredentialsGetter, caPEMs [][]byte) error {
	m.replaceCAsCalled++
	if m.replaceCAs != nil {
		return m.replaceCAs(config, getCreds, caPEMs)
	}
	return nil
}

var (
	testConfig = ServiceClientConfig{
		Addr: "foo:123",
		CA:   "/foo/bar.pem",
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
			certSentinel := new(tls.Certificate)
			registrator := &mockRegistrator{
				register: func(config ServiceClientConfig, getCreds ClientCredentialsGetter, hostCAPem []byte, name string) error {
					require.Equal(t, testConfig, config)
					c, err := getCreds()
					require.NoError(t, err)
					require.Same(t, certSentinel, c)
					require.Equal(t, ca.GetActiveKeys().TLS[0].Cert, hostCAPem)
					require.Equal(t, clusterName.GetClusterName(), name)
					return tc.registerError
				},
				replaceCAs: func(config ServiceClientConfig, getCreds ClientCredentialsGetter, caPEMs [][]byte) error {
					require.Equal(t, testConfig, config)
					c, err := getCreds()
					require.NoError(t, err)
					require.Same(t, certSentinel, c)
					require.Equal(t, [][]byte{ca.GetActiveKeys().TLS[0].Cert}, caPEMs)
					return tc.replaceCAsError
				},
			}
			err := Register(ctx, registrator, testConfig, func() (*tls.Certificate, error) {
				return certSentinel, nil
			}, auth, nil)
			require.Equal(t, 1, registrator.registerCalled)
			require.Equal(t, 1, registrator.replaceCAsCalled)

			if tc.registerError != nil && tc.replaceCAsError != nil {
				require.Error(t, err)
				unwrapped := trace.Unwrap(err)
				var aerr trace.Aggregate
				require.ErrorAs(t, unwrapped, &aerr, "expected error to be trace.Aggregate, was %T instead", unwrapped)
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
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	licenseKeyPair, err := liblicense.ParseLicensePEM([]byte(fixtures.TestLicenseData))
	require.NoError(t, err)

	ctx := context.Background()

	ca := createCA(t, "", nil)
	clusterName := createClusterName(t)
	auth := &mockAuth{
		ca:          ca,
		clusterName: clusterName,
	}
	license := &licensefile.LicenseFile{
		KeyPair: licenseKeyPair,
	}

	certSentinel := new(tls.Certificate)
	registrator := &mockRegistrator{
		register: func(config ServiceClientConfig, getCreds ClientCredentialsGetter, hostCAPem []byte, name string) error {
			require.Equal(t, testConfig, config)
			c, err := getCreds()
			require.NoError(t, err)
			require.NotSame(t, certSentinel, c)
			return nil
		},
		replaceCAs: func(config ServiceClientConfig, getCreds ClientCredentialsGetter, caPEMs [][]byte) error {
			require.Equal(t, testConfig, config)
			c, err := getCreds()
			require.NoError(t, err)
			require.Same(t, certSentinel, c)
			return nil
		},
	}
	err = Register(ctx, registrator, testConfig, func() (*tls.Certificate, error) {
		return certSentinel, nil
	}, auth, license)
	require.NoError(t, err)
	require.Equal(t, 1, registrator.registerCalled)
	require.Equal(t, 1, registrator.replaceCAsCalled)
}

func TestRegister_CARotation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

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

			certSentinel := new(tls.Certificate)
			registrator := &mockRegistrator{
				register: func(config ServiceClientConfig, getCreds ClientCredentialsGetter, hostCAPem []byte, name string) error {
					require.Equal(t, testConfig, config)
					c, err := getCreds()
					require.NoError(t, err)
					require.Same(t, certSentinel, c)
					require.Equal(t, tc.expectedRegisterCA, hostCAPem)
					return nil
				},
				replaceCAs: func(config ServiceClientConfig, getCreds ClientCredentialsGetter, caPEMs [][]byte) error {
					require.Equal(t, testConfig, config)
					c, err := getCreds()
					require.NoError(t, err)
					require.Same(t, certSentinel, c)
					require.Equal(t, tc.expectedCAs, caPEMs)
					return nil
				},
			}
			err := Register(ctx, registrator, testConfig, func() (*tls.Certificate, error) {
				return certSentinel, nil
			}, auth, nil)
			require.NoError(t, err)
			require.Equal(t, 1, registrator.registerCalled)
			require.Equal(t, 1, registrator.replaceCAsCalled)
		})
	}
}
