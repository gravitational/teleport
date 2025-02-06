/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/proxy"
)

// TestReadIdentity makes parses identity from private key and certificate
// and checks that all parameters are valid
func TestReadIdentity(t *testing.T) {
	clock := clockwork.NewFakeClock()
	a := testauthority.NewWithClock(clock)
	priv, pub, err := a.GenerateKeyPair()
	require.NoError(t, err)
	caSigner, err := ssh.ParsePrivateKey(priv)
	require.NoError(t, err)

	cert, err := a.GenerateHostCert(sshca.HostCertificateRequest{
		CASigner:      caSigner,
		PublicHostKey: pub,
		HostID:        "id1",
		NodeName:      "node-name",
		TTL:           0,
		Identity: sshca.Identity{
			ClusterName: "example.com",
			SystemRole:  types.RoleNode,
		},
	})
	require.NoError(t, err)

	id, err := state.ReadSSHIdentityFromKeyPair(priv, cert)
	require.NoError(t, err)
	require.Equal(t, "example.com", id.ClusterName)
	require.Equal(t, state.IdentityID{HostUUID: "id1.example.com", Role: types.RoleNode}, id.ID)
	require.Equal(t, cert, id.CertBytes)
	require.Equal(t, priv, id.KeyBytes)

	// test TTL by converting the generated cert to text -> back and making sure ExpireAfter is valid
	ttl := 10 * time.Second
	expiryDate := clock.Now().Add(ttl)
	bytes, err := a.GenerateHostCert(sshca.HostCertificateRequest{
		CASigner:      caSigner,
		PublicHostKey: pub,
		HostID:        "id1",
		NodeName:      "node-name",
		TTL:           ttl,
		Identity: sshca.Identity{
			ClusterName: "example.com",
			SystemRole:  types.RoleNode,
		},
	})
	require.NoError(t, err)
	copy, err := apisshutils.ParseCertificate(bytes)
	require.NoError(t, err)
	require.Equal(t, uint64(expiryDate.Unix()), copy.ValidBefore)
}

func TestBadIdentity(t *testing.T) {
	a := testauthority.New()
	priv, pub, err := a.GenerateKeyPair()
	require.NoError(t, err)
	caSigner, err := ssh.ParsePrivateKey(priv)
	require.NoError(t, err)

	// bad cert type
	_, err = state.ReadSSHIdentityFromKeyPair(priv, pub)
	require.IsType(t, trace.BadParameter(""), err)

	// missing authority domain
	cert, err := a.GenerateHostCert(sshca.HostCertificateRequest{
		CASigner:      caSigner,
		PublicHostKey: pub,
		HostID:        "id2",
		NodeName:      "",
		TTL:           0,
		Identity: sshca.Identity{
			ClusterName: "",
			SystemRole:  types.RoleNode,
		},
	})
	require.NoError(t, err)

	_, err = state.ReadSSHIdentityFromKeyPair(priv, cert)
	require.IsType(t, trace.BadParameter(""), err)

	// missing host uuid
	cert, err = a.GenerateHostCert(sshca.HostCertificateRequest{
		CASigner:      caSigner,
		PublicHostKey: pub,
		HostID:        "example.com",
		NodeName:      "",
		TTL:           0,
		Identity: sshca.Identity{
			ClusterName: "",
			SystemRole:  types.RoleNode,
		},
	})
	require.NoError(t, err)

	_, err = state.ReadSSHIdentityFromKeyPair(priv, cert)
	require.IsType(t, trace.BadParameter(""), err)

	// unrecognized role
	cert, err = a.GenerateHostCert(sshca.HostCertificateRequest{
		CASigner:      caSigner,
		PublicHostKey: pub,
		HostID:        "example.com",
		NodeName:      "",
		TTL:           0,
		Identity: sshca.Identity{
			ClusterName: "id1",
			SystemRole:  "bad role",
		},
	})
	require.NoError(t, err)

	_, err = state.ReadSSHIdentityFromKeyPair(priv, cert)
	require.IsType(t, trace.BadParameter(""), err)
}

func TestSignatureAlgorithmSuite(t *testing.T) {
	ctx := context.Background()

	suiteName := func(suite types.SignatureAlgorithmSuite) string {
		suiteName, err := suite.MarshalText()
		require.NoError(t, err)
		return string(suiteName)
	}

	assertSuitesEqual := func(t *testing.T, expected, actual types.SignatureAlgorithmSuite) {
		t.Helper()
		assert.Equal(t, suiteName(expected), suiteName(actual))
	}

	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.HSM: {Enabled: true},
			},
		},
	})

	setupInitConfig := func(t *testing.T, capOrigin string, fips, hsm bool) InitConfig {
		cfg := setupConfig(t)
		cfg.FIPS = fips
		if hsm {
			cfg.KeyStoreConfig = keystore.HSMTestConfig(t)
		}
		cfg.AuthPreference.SetOrigin(capOrigin)
		if capOrigin != types.OriginDefaults {
			cfg.AuthPreference.SetSignatureAlgorithmSuite(types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED)
		}
		// Pre-generate all CAs to keep tests fast esp. with SoftHSM.
		for _, caType := range types.CertAuthTypes {
			cfg.BootstrapResources = append(cfg.BootstrapResources, suite.NewTestCAWithConfig(suite.TestCAConfig{
				Type:        caType,
				ClusterName: cfg.ClusterName.GetClusterName(),
				Clock:       cfg.Clock,
			}))
		}
		return cfg
	}

	testCases := map[string]struct {
		fips                  bool
		hsm                   bool
		cloud                 bool
		expectDefaultSuite    types.SignatureAlgorithmSuite
		expectUnallowedSuites []types.SignatureAlgorithmSuite
	}{
		"basic": {
			expectDefaultSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
		},
		"fips": {
			fips:               true,
			expectDefaultSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
			expectUnallowedSuites: []types.SignatureAlgorithmSuite{
				types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
				types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
			},
		},
		"hsm": {
			hsm:                true,
			expectDefaultSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
			expectUnallowedSuites: []types.SignatureAlgorithmSuite{
				types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
			},
		},
		"fips and hsm": {
			fips:               true,
			hsm:                true,
			expectDefaultSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
			expectUnallowedSuites: []types.SignatureAlgorithmSuite{
				types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
				types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
			},
		},
		"cloud": {
			cloud:              true,
			expectDefaultSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
			expectUnallowedSuites: []types.SignatureAlgorithmSuite{
				types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
			},
		},
	}

	// Test the behavior of auth server init. A default signature algorithm
	// suite should never overwrite a persisted signature algorithm suite for an
	// existing cluster, even if that was also a default.
	t.Run("init", func(t *testing.T) {
		for _, origin := range []string{types.OriginDefaults, types.OriginConfigFile} {
			t.Run(origin, func(t *testing.T) {
				for desc, tc := range testCases {
					t.Run(desc, func(t *testing.T) {
						if tc.cloud {
							modules.SetTestModules(t, &modules.TestModules{
								TestFeatures: modules.Features{
									Cloud: true,
									Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
										entitlements.HSM: {Enabled: true},
									},
								},
							})
						}

						// Assert that a fresh cluster with no signature_algorithm_suite
						// configured gets the expected default suite, whether
						// or not anything else in the cluster auth preference is set.
						cfg := setupInitConfig(t, origin, tc.fips, tc.hsm)
						auth1, err := Init(ctx, cfg)
						require.NoError(t, err)
						t.Cleanup(func() { auth1.Close() })
						authPref, err := auth1.GetAuthPreference(ctx)
						require.NoError(t, err)
						assert.Equal(t, origin, authPref.GetMetadata().Labels[types.OriginLabel])
						assertSuitesEqual(t, tc.expectDefaultSuite, authPref.GetSignatureAlgorithmSuite())

						// Start a second auth server with the same backend and
						// config, assert that the default suite remains.
						auth2, err := Init(ctx, cfg)
						require.NoError(t, err)
						t.Cleanup(func() { auth2.Close() })
						authPref, err = auth2.GetAuthPreference(ctx)
						require.NoError(t, err)
						assert.Equal(t, origin, authPref.GetMetadata().Labels[types.OriginLabel])
						assertSuitesEqual(t, tc.expectDefaultSuite, authPref.GetSignatureAlgorithmSuite())

						// In the stored cluster_auth_preference, reset the
						// signature_algorithm_suite to unspecified (still with
						// the same origin) to mimic an older cluster with the old
						// defaults, in the next step it will be "upgraded".
						authPref.SetSignatureAlgorithmSuite(types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED)
						_, err = auth2.UpsertAuthPreference(ctx, authPref)
						require.NoError(t, err)
						authPref, err = auth2.GetAuthPreference(ctx)
						require.NoError(t, err)
						// Sanity check it persisted.
						assert.Equal(t, origin, authPref.GetMetadata().Labels[types.OriginLabel])
						assertSuitesEqual(t, types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED, authPref.GetSignatureAlgorithmSuite())

						// Start a third brand new auth server sharing the same
						// backend and config. The new auth starting up should
						// apply the new default auth preference and persist it
						// to the backend, but it should not modify the existing
						// signature algorithm suite even though it's
						// unspecified. This is meant to test that a v16 auth
						// server upgraded to v17 will still have an unspecified
						// signature algorithm suite and won't get a new one
						// until explicitly opting in.
						auth3, err := Init(ctx, cfg)
						require.NoError(t, err)
						t.Cleanup(func() { auth3.Close() })
						authPref, err = auth3.GetAuthPreference(ctx)
						require.NoError(t, err)
						assert.Equal(t, origin, authPref.GetMetadata().Labels[types.OriginLabel])
						assertSuitesEqual(t, types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED, authPref.GetSignatureAlgorithmSuite())

						// Assert that the selected algorithm is RSA2048 when the suite is
						// unspecified.
						alg, err := cryptosuites.AlgorithmForKey(ctx,
							cryptosuites.GetCurrentSuiteFromAuthPreference(auth3),
							cryptosuites.UserTLS)
						require.NoError(t, err)
						assert.Equal(t, cryptosuites.RSA2048.String(), alg.String())
					})
				}
			})
		}
	})

	// Test that the auth preference cannot be upserted with a signature
	// algorithm suite incompatible with the cluster FIPS and HSM settings.
	t.Run("upsert", func(t *testing.T) {
		for desc, tc := range testCases {
			t.Run(desc, func(t *testing.T) {
				if tc.cloud {
					modules.SetTestModules(t, &modules.TestModules{
						TestFeatures: modules.Features{
							Cloud: true,
							Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
								entitlements.HSM: {Enabled: true},
							},
						},
					})
				}
				cfg := TestAuthServerConfig{
					Dir:  t.TempDir(),
					FIPS: tc.fips,
					AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
						// Cloud requires second factor enabled.
						SecondFactor: constants.SecondFactorOn,
						Webauthn: &types.Webauthn{
							RPID: "teleport.example.com",
						},
					},
				}
				if tc.hsm {
					cfg.KeystoreConfig = keystore.HSMTestConfig(t)
				}
				testAuthServer, err := NewTestAuthServer(cfg)
				require.NoError(t, err)
				tlsServer, err := testAuthServer.NewTestTLSServer()
				require.NoError(t, err)
				clt, err := tlsServer.NewClient(TestAdmin())
				require.NoError(t, err)

				for _, suiteValue := range types.SignatureAlgorithmSuite_value {
					suite := types.SignatureAlgorithmSuite(suiteValue)
					t.Run(suiteName(suite), func(t *testing.T) {
						authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
							SignatureAlgorithmSuite: suite,
						})
						require.NoError(t, err)

						_, err = clt.UpsertAuthPreference(ctx, authPref)
						if slices.Contains(tc.expectUnallowedSuites, suite) {
							var badParameterErr *trace.BadParameterError
							assert.ErrorAs(t, err, &badParameterErr)
							return
						} else {
							assert.NoError(t, err)
						}

						// Reset should go back to the default suite.
						err = clt.ResetAuthPreference(ctx)
						require.NoError(t, err)
						authPref, err = clt.GetAuthPreference(ctx)
						require.NoError(t, err)
						assert.Equal(t, types.OriginDefaults, authPref.GetMetadata().Labels[types.OriginLabel])
						assertSuitesEqual(t, tc.expectDefaultSuite, authPref.GetSignatureAlgorithmSuite())
					})
				}
			})
		}
	})
}

type testDynamicallyConfigurableParams struct {
	withDefaults, withConfigFile, withAnotherConfigFile func(*testing.T, *InitConfig) types.ResourceWithOrigin
	setDynamic                                          func(*testing.T, *Server)
	getStored                                           func(*testing.T, *Server) types.ResourceWithOrigin
}

func testDynamicallyConfigurable(t *testing.T, p testDynamicallyConfigurableParams) {
	initAuthServer := func(t *testing.T, conf InitConfig) *Server {
		authServer, err := Init(context.Background(), conf)
		require.NoError(t, err)
		t.Cleanup(func() { authServer.Close() })
		return authServer
	}

	t.Run("start with config file, reinit with defaults", func(t *testing.T) {
		t.Parallel()
		conf := setupConfig(t)

		// Simulate a server with a config-file resource.
		configFileRes := p.withConfigFile(t, &conf)
		authServer := initAuthServer(t, conf)

		stored := p.getStored(t, authServer)
		require.Equal(t, types.OriginConfigFile, stored.Origin())
		require.Empty(t, resourceDiff(configFileRes, stored))

		// Reinitialize with the default resource.
		defaultRes := p.withDefaults(t, &conf)
		authServer = initAuthServer(t, conf)

		// Verify the stored resource is now labeled as originating from defaults.
		stored = p.getStored(t, authServer)
		require.Equal(t, types.OriginDefaults, stored.Origin())
		require.Empty(t, resourceDiff(defaultRes, stored))
	})

	t.Run("start with dynamic, reinit with defaults", func(t *testing.T) {
		t.Parallel()
		conf := setupConfig(t)

		// Simulate a server with dynamic configuration.
		authServer := initAuthServer(t, conf)
		p.setDynamic(t, authServer)

		dynamic := p.getStored(t, authServer)
		require.Equal(t, types.OriginDynamic, dynamic.Origin())

		// Attempt to reinitialize with the default resource should be a no-op.
		p.withDefaults(t, &conf)
		authServer = initAuthServer(t, conf)

		// Verify the stored resource remains unchanged.
		stored := p.getStored(t, authServer)
		require.Equal(t, types.OriginDynamic, stored.Origin())
		require.Empty(t, resourceDiff(dynamic, stored))
	})

	t.Run("start with dynamic, reinit with config file", func(t *testing.T) {
		t.Parallel()
		conf := setupConfig(t)

		// Simulate a server with dynamic configuration.
		authServer := initAuthServer(t, conf)
		p.setDynamic(t, authServer)

		dynamic := p.getStored(t, authServer)
		require.Equal(t, types.OriginDynamic, dynamic.Origin())

		// Reinitialize with a config-file resource.
		configFileRes := p.withConfigFile(t, &conf)
		authServer = initAuthServer(t, conf)

		// Verify the stored resource is updated.
		stored := p.getStored(t, authServer)
		require.Equal(t, types.OriginConfigFile, stored.Origin())
		require.Empty(t, resourceDiff(configFileRes, stored))
	})

	t.Run("start with defaults, reinit with config file", func(t *testing.T) {
		t.Parallel()
		conf := setupConfig(t)

		// Simulate a server with the default resource.
		defaultRes := p.withDefaults(t, &conf)
		authServer := initAuthServer(t, conf)

		stored := p.getStored(t, authServer)
		require.Equal(t, types.OriginDefaults, stored.Origin())
		require.Empty(t, resourceDiff(defaultRes, stored))

		// Reinitialize with a config-file resource.
		configFileRes := p.withConfigFile(t, &conf)
		authServer = initAuthServer(t, conf)

		// Verify the stored resource is updated.
		stored = p.getStored(t, authServer)
		require.Equal(t, types.OriginConfigFile, stored.Origin())
		require.Empty(t, resourceDiff(configFileRes, stored))
	})

	t.Run("start with config file, reinit with another config file", func(t *testing.T) {
		t.Parallel()
		conf := setupConfig(t)

		// Simulate a server with a config-file resource.
		configFileRes := p.withConfigFile(t, &conf)
		authServer := initAuthServer(t, conf)

		stored := p.getStored(t, authServer)
		require.Equal(t, types.OriginConfigFile, stored.Origin())
		require.Empty(t, resourceDiff(configFileRes, stored))

		// Reinitialize with another config-file resource.
		anotherConfigFileRes := p.withAnotherConfigFile(t, &conf)
		authServer = initAuthServer(t, conf)

		// Verify the stored resource is updated.
		stored = p.getStored(t, authServer)
		require.Equal(t, types.OriginConfigFile, stored.Origin())
		require.Empty(t, resourceDiff(anotherConfigFileRes, stored))
	})
}

func TestAuthPreference(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testDynamicallyConfigurable(t, testDynamicallyConfigurableParams{
		withDefaults: func(t *testing.T, conf *InitConfig) types.ResourceWithOrigin {
			conf.AuthPreference = types.DefaultAuthPreference()
			return conf.AuthPreference
		},
		withConfigFile: func(t *testing.T, conf *InitConfig) types.ResourceWithOrigin {
			fromConfigFile, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
				Type:                    constants.OIDC,
				SignatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
			})
			require.NoError(t, err)
			conf.AuthPreference = fromConfigFile
			return conf.AuthPreference
		},
		withAnotherConfigFile: func(t *testing.T, conf *InitConfig) types.ResourceWithOrigin {
			conf.AuthPreference = newWebauthnAuthPreferenceConfigFromFile(t)
			conf.AuthPreference.SetSignatureAlgorithmSuite(types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1)
			return conf.AuthPreference
		},
		setDynamic: func(t *testing.T, authServer *Server) {
			dynamically, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
				SecondFactor: constants.SecondFactorOff,
			})
			require.NoError(t, err)
			_, err = authServer.UpsertAuthPreference(ctx, dynamically)
			require.NoError(t, err)
		},
		getStored: func(t *testing.T, authServer *Server) types.ResourceWithOrigin {
			authPref, err := authServer.GetAuthPreference(ctx)
			require.NoError(t, err)
			return authPref
		},
	})
}

func TestAuthPreferenceSecondFactorOnly(t *testing.T) {
	modules.SetInsecureTestMode(false)
	defer modules.SetInsecureTestMode(true)
	ctx := context.Background()

	t.Run("starting with second_factor disabled fails", func(t *testing.T) {
		conf := setupConfig(t)
		authPref, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
			SecondFactor: constants.SecondFactorOff,
		})
		require.NoError(t, err)

		conf.AuthPreference = authPref
		_, err = Init(ctx, conf)
		require.Error(t, err)
	})

	t.Run("starting with defaults and dynamically updating to disable second factor fails", func(t *testing.T) {
		conf := setupConfig(t)
		s, err := Init(ctx, conf)
		require.NoError(t, err)
		authpref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
			SecondFactor: constants.SecondFactorOff,
		})
		require.NoError(t, err)
		_, err = s.UpsertAuthPreference(ctx, authpref)
		require.Error(t, err)
	})
}

func TestClusterNetworkingConfig(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testDynamicallyConfigurable(t, testDynamicallyConfigurableParams{
		withDefaults: func(t *testing.T, conf *InitConfig) types.ResourceWithOrigin {
			conf.ClusterNetworkingConfig = types.DefaultClusterNetworkingConfig()
			return conf.ClusterNetworkingConfig
		},
		withConfigFile: func(t *testing.T, conf *InitConfig) types.ResourceWithOrigin {
			fromConfigFile, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
				ClientIdleTimeout: types.Duration(7 * time.Minute),
			})
			require.NoError(t, err)
			conf.ClusterNetworkingConfig = fromConfigFile
			return conf.ClusterNetworkingConfig
		},
		withAnotherConfigFile: func(t *testing.T, conf *InitConfig) types.ResourceWithOrigin {
			anotherFromConfigFile, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
				ClientIdleTimeout: types.Duration(10 * time.Minute),
				KeepAliveInterval: types.Duration(3 * time.Minute),
			})
			require.NoError(t, err)
			conf.ClusterNetworkingConfig = anotherFromConfigFile
			return conf.ClusterNetworkingConfig
		},
		setDynamic: func(t *testing.T, authServer *Server) {
			dynamically, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
				KeepAliveInterval: types.Duration(4 * time.Minute),
			})
			require.NoError(t, err)
			dynamically.SetOrigin(types.OriginDynamic)
			_, err = authServer.UpsertClusterNetworkingConfig(ctx, dynamically)
			require.NoError(t, err)
		},
		getStored: func(t *testing.T, authServer *Server) types.ResourceWithOrigin {
			authPref, err := authServer.GetClusterNetworkingConfig(ctx)
			require.NoError(t, err)
			return authPref
		},
	})
}

func TestSessionRecordingConfig(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testDynamicallyConfigurable(t, testDynamicallyConfigurableParams{
		withDefaults: func(t *testing.T, conf *InitConfig) types.ResourceWithOrigin {
			conf.SessionRecordingConfig = types.DefaultSessionRecordingConfig()
			return conf.SessionRecordingConfig
		},
		withConfigFile: func(t *testing.T, conf *InitConfig) types.ResourceWithOrigin {
			fromConfigFile, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
				Mode: types.RecordOff,
			})
			require.NoError(t, err)
			conf.SessionRecordingConfig = fromConfigFile
			return conf.SessionRecordingConfig
		},
		withAnotherConfigFile: func(t *testing.T, conf *InitConfig) types.ResourceWithOrigin {
			anotherFromConfigFile, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
				Mode: types.RecordAtProxySync,
			})
			require.NoError(t, err)
			conf.SessionRecordingConfig = anotherFromConfigFile
			return conf.SessionRecordingConfig
		},
		setDynamic: func(t *testing.T, authServer *Server) {
			dynamically, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
				Mode: types.RecordAtNodeSync,
			})
			require.NoError(t, err)
			dynamically.SetOrigin(types.OriginDynamic)
			_, err = authServer.UpsertSessionRecordingConfig(ctx, dynamically)
			require.NoError(t, err)
		},
		getStored: func(t *testing.T, authServer *Server) types.ResourceWithOrigin {
			authPref, err := authServer.GetSessionRecordingConfig(ctx)
			require.NoError(t, err)
			return authPref
		},
	})
}

func TestClusterID(t *testing.T) {
	conf := setupConfig(t)
	authServer, err := Init(context.Background(), conf)
	require.NoError(t, err)
	defer authServer.Close()

	cc, err := authServer.GetClusterName()
	require.NoError(t, err)
	clusterID := cc.GetClusterID()
	require.NotEmpty(t, clusterID)

	// do it again and make sure cluster ID hasn't changed
	authServer, err = Init(context.Background(), conf)
	require.NoError(t, err)
	defer authServer.Close()

	cc, err = authServer.GetClusterName()
	require.NoError(t, err)
	require.Equal(t, clusterID, cc.GetClusterID())
}

// TestClusterName ensures that a cluster can not be renamed.
func TestClusterName(t *testing.T) {
	conf := setupConfig(t)
	authServer, err := Init(context.Background(), conf)
	require.NoError(t, err)
	defer authServer.Close()

	// Start the auth server with a different cluster name. The auth server
	// should start, but with the original name.
	newConfig := conf
	newConfig.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "dev.localhost",
	})
	require.NoError(t, err)
	authServer, err = Init(context.Background(), newConfig)
	require.NoError(t, err)
	defer authServer.Close()

	cn, err := authServer.GetClusterName()
	require.NoError(t, err)
	require.NotEqual(t, newConfig.ClusterName.GetClusterName(), cn.GetClusterName())
	require.Equal(t, conf.ClusterName.GetClusterName(), cn.GetClusterName())
}

func keysIn[K comparable, V any](m map[K]V) []K {
	result := make([]K, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

type failingTrustInternal struct {
	services.TrustInternal
}

func (t *failingTrustInternal) CreateCertAuthority(ctx context.Context, ca types.CertAuthority) error {
	return trace.Errorf("error")
}

// TestInitCertFailureRecovery ensures the auth server is able to recover from
// a failure in the cert creation process.
func TestInitCertFailureRecovery(t *testing.T) {
	ctx := context.Background()
	cap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type: constants.SAML,
	})
	require.NoError(t, err)

	conf := setupConfig(t)

	// BootstrapResources have lead to an unrecoverable state in the past.
	// See https://github.com/gravitational/teleport/pull/49638.
	conf.BootstrapResources = []types.Resource{cap}
	_, err = Init(ctx, conf, func(s *Server) error {
		s.TrustInternal = &failingTrustInternal{
			TrustInternal: s.TrustInternal,
		}
		return nil
	})
	require.Error(t, err)

	_, err = Init(ctx, conf)
	require.NoError(t, err)
}

// TestPresets tests behavior of presets
func TestPresets(t *testing.T) {
	ctx := context.Background()

	presetRoleNames := []string{
		teleport.PresetEditorRoleName,
		teleport.PresetAccessRoleName,
		teleport.PresetAuditorRoleName,
		teleport.PresetTerraformProviderRoleName,
		teleport.PresetWildcardWorkloadIdentityIssuerRoleName,
	}

	t.Run("EmptyCluster", func(t *testing.T) {
		as := newTestAuthServer(ctx, t)
		clock := clockwork.NewFakeClock()
		as.SetClock(clock)

		err := createPresetRoles(ctx, as)
		require.NoError(t, err)

		// Second call should not fail
		err = createPresetRoles(ctx, as)
		require.NoError(t, err)

		// Presets were created
		for _, role := range presetRoleNames {
			_, err := as.GetRole(ctx, role)
			require.NoError(t, err)
		}
	})

	// Makes sure that existing role with the same name is not modified
	t.Run("ExistingRole", func(t *testing.T) {
		as := newTestAuthServer(ctx, t)
		clock := clockwork.NewFakeClock()
		as.SetClock(clock)

		access := services.NewPresetEditorRole()
		access.SetLogins(types.Allow, []string{"root"})
		access, err := as.CreateRole(ctx, access)
		require.NoError(t, err)

		err = createPresetRoles(ctx, as)
		require.NoError(t, err)

		// Presets were created
		for _, role := range presetRoleNames {
			_, err := as.GetRole(ctx, role)
			require.NoError(t, err)
		}

		out, err := as.GetRole(ctx, access.GetName())
		require.NoError(t, err)
		require.Equal(t, access.GetLogins(types.Allow), out.GetLogins(types.Allow))
	})

	// If a default allow condition is not present, ensure it gets added.
	t.Run("AddDefaultAllowConditions", func(t *testing.T) {
		as := newTestAuthServer(ctx, t)
		clock := clockwork.NewFakeClock()
		as.SetClock(clock)

		editorRole := services.NewPresetEditorRole()
		rules := editorRole.GetRules(types.Allow)

		// Create a new set of rules based on the Editor Role, excluding the ConnectionDiagnostic.
		// ConnectionDiagnostic is part of the default allow rules
		var outdatedRules []types.Rule
		for _, r := range rules {
			if slices.Contains(r.Resources, types.KindConnectionDiagnostic) {
				continue
			}
			outdatedRules = append(outdatedRules, r)
		}
		editorRole.SetRules(types.Allow, outdatedRules)
		editorRole, err := as.CreateRole(ctx, editorRole)
		require.NoError(t, err)

		// Set up an old Access Role.
		// Remove the new DatabaseServiceLabels default
		accessRole := services.NewPresetAccessRole()
		accessRole.SetDatabaseServiceLabels(types.Allow, types.Labels{})
		accessRole, err = as.CreateRole(ctx, accessRole)
		require.NoError(t, err)

		err = createPresetRoles(ctx, as)
		require.NoError(t, err)

		outEditor, err := as.GetRole(ctx, editorRole.GetName())
		require.NoError(t, err)

		allowRules := outEditor.GetRules(types.Allow)
		require.Condition(t, func() (success bool) {
			for _, r := range allowRules {
				if slices.Contains(r.Resources, types.KindConnectionDiagnostic) {
					return true
				}
			}
			return false
		}, "missing default rule")

		outAccess, err := as.GetRole(ctx, accessRole.GetName())
		require.NoError(t, err)
		allowedDatabaseServiceLabels := outAccess.GetDatabaseServiceLabels(types.Allow)
		require.Equal(t, types.Labels{types.Wildcard: []string{types.Wildcard}}, allowedDatabaseServiceLabels, "missing default DatabaseServiceLabels")
	})

	// Don't set a default allow rule if the resource is present in the role.
	// Either as part of allowing or denying rules.
	t.Run("DefaultAllowRulesNotAppliedIfExplicitlyDefined", func(t *testing.T) {
		as := newTestAuthServer(ctx, t)
		clock := clockwork.NewFakeClock()
		as.SetClock(clock)

		// Set up a changed Editor Role
		editorRole := services.NewPresetEditorRole()
		allowRules := editorRole.GetRules(types.Allow)

		// Create a new set of rules based on the Editor Role,
		// setting a deny rule for a default allow rule
		var outdateAllowRules []types.Rule
		for _, r := range allowRules {
			if slices.Contains(r.Resources, types.KindConnectionDiagnostic) {
				continue
			}
			outdateAllowRules = append(outdateAllowRules, r)
		}
		editorRole.SetRules(types.Allow, outdateAllowRules)

		// Explicitly deny Create to ConnectionDiagnostic
		denyRules := editorRole.GetRules(types.Deny)
		denyConnectionDiagnosticRule := types.NewRule(types.KindConnectionDiagnostic, []string{types.VerbCreate})
		denyRules = append(denyRules, denyConnectionDiagnosticRule)
		editorRole.SetRules(types.Deny, denyRules)

		editorRole, err := as.CreateRole(ctx, editorRole)
		require.NoError(t, err)

		// Set up a changed Access Role
		accessRole := services.NewPresetAccessRole()
		// Remove a default allow label as well.
		accessRole.SetDatabaseServiceLabels(types.Allow, types.Labels{})
		// Explicitly deny DatabaseServiceLabels
		accessRole.SetDatabaseServiceLabels(types.Deny, types.Labels{types.Wildcard: []string{types.Wildcard}})

		accessRole, err = as.CreateRole(ctx, accessRole)
		require.NoError(t, err)

		// Apply defaults.
		err = createPresetRoles(ctx, as)
		require.NoError(t, err)

		outEditor, err := as.GetRole(ctx, editorRole.GetName())
		require.NoError(t, err)

		allowRules = outEditor.GetRules(types.Allow)
		require.Condition(t, func() (success bool) {
			for _, r := range allowRules {
				if slices.Contains(r.Resources, types.KindConnectionDiagnostic) {
					return false
				}
			}
			return true
		}, "missing default rule")

		outAccess, err := as.GetRole(ctx, accessRole.GetName())
		require.NoError(t, err)
		allowedDatabaseServiceLabels := outAccess.GetDatabaseServiceLabels(types.Allow)
		require.Nil(t, allowedDatabaseServiceLabels, "does not set Allowed DatabaseService Labels")
		deniedDatabaseServiceLabels := outAccess.GetDatabaseServiceLabels(types.Deny)
		require.Equal(t, types.Labels{types.Wildcard: []string{types.Wildcard}}, deniedDatabaseServiceLabels, "keeps the deny label for DatabaseService")
	})

	upsertRoleTest := func(t *testing.T, expectedPresetRoles []string, expectedSystemRoles []string) {
		// test state
		ctx := context.Background()
		// mu protects created resource maps
		var mu sync.Mutex
		createdSystemRoles := make(map[string]types.Role)
		createdPresets := make(map[string]types.Role)

		//
		// Test #1 - populating an empty cluster
		//
		roleManager := newMockRoleManager(t)

		// EXPECT that non-system resources will be created once
		// and once only.
		roleManager.
			On("CreateRole", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				r := args[1].(types.Role)
				mu.Lock()
				defer mu.Unlock()
				require.Contains(t, expectedPresetRoles, r.GetName())
				require.NotContains(t, createdPresets, r.GetName())
				require.False(t, types.IsSystemResource(r) && r.GetName() != teleport.SystemOktaRequesterRoleName)
				createdPresets[r.GetName()] = r
			}).
			Return(func(_ context.Context, r types.Role) (types.Role, error) {
				return r, nil
			})

		// EXPECT that any (and ONLY) system resources will be upserted
		roleManager.
			On("UpsertRole", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				r := args[1].(types.Role)
				mu.Lock()
				defer mu.Unlock()
				require.True(t, types.IsSystemResource(r))
				require.Contains(t, expectedSystemRoles, r.GetName())
				require.NotContains(t, keysIn(createdSystemRoles), r.GetName())
				createdSystemRoles[r.GetName()] = r
			}).
			Maybe().
			Return(func(_ context.Context, r types.Role) (types.Role, error) {
				return r, nil
			})

		err := createPresetRoles(ctx, roleManager)
		require.NoError(t, err)
		require.ElementsMatch(t, keysIn(createdPresets), expectedPresetRoles)
		require.ElementsMatch(t, keysIn(createdSystemRoles), expectedSystemRoles)
		roleManager.AssertExpectations(t)

		//
		// Test #2 - populating an already-populated cluster
		//
		roleManager = newMockRoleManager(t)

		// EXPECT that createPresets will try to create all expected
		// non-system roles
		roleManager.
			On("CreateRole", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				mu.Lock()
				defer mu.Unlock()
				require.Contains(t, createdPresets, args[1].(types.Role).GetName())
			}).
			Return(nil, trace.AlreadyExists("dupe"))

		// EXPECT that any (and ONLY) expected system roles will be
		// automatically upserted
		roleManager.
			On("UpsertRole", mock.Anything, mock.Anything).
			Run(requireSystemResource(t, 1)).
			Maybe().
			Return(func(_ context.Context, r types.Role) (types.Role, error) {
				return r, nil
			})

		// EXPECT that all of the roles created in the previous step (and ONLY the
		// roles created in the previous step will be queried.
		for name, role := range createdPresets {
			roleManager.
				On("GetRole", mock.Anything, name).
				Return(role, nil)
		}

		err = createPresetRoles(ctx, roleManager)
		require.NoError(t, err)
		roleManager.AssertExpectations(t)

		//
		// Test #3 - populating an already-populated cluster with updated presets
		//
		roleManager = newMockRoleManager(t)

		// Removing a specific resource which is part of the Default Allow Rules
		// should trigger an UpsertRole call
		editorRole := createdPresets[teleport.PresetEditorRoleName]
		var allowRulesWithoutConnectionDiag []types.Rule

		for _, r := range editorRole.GetRules(types.Allow) {
			if slices.Contains(r.Resources, types.KindConnectionDiagnostic) {
				continue
			}
			allowRulesWithoutConnectionDiag = append(allowRulesWithoutConnectionDiag, r)
		}
		editorRole.SetRules(types.Allow, allowRulesWithoutConnectionDiag)

		// EXPECT that createPresets will try to create all expected
		// non-system roles
		remainingPresets := toSet(expectedPresetRoles)
		roleManager.
			On("CreateRole", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				mu.Lock()
				defer mu.Unlock()
				r := args[1].(types.Role)
				require.Contains(t, createdPresets, r.GetName())
				delete(remainingPresets, r.GetName())
			}).
			Return(nil, trace.AlreadyExists("dupe"))

		// EXPECT that all of the roles created in the first step (and ONLY the
		// roles created in the first step will be queried.
		for name, role := range createdPresets {
			roleManager.
				On("GetRole", mock.Anything, name).
				Return(role, nil)
		}

		// EXPECT that any system roles will be automatically upserted
		// AND our modified editor resource will be updated using an upsert
		roleManager.
			On("UpsertRole", mock.Anything, mock.Anything).
			Return(func(_ context.Context, r types.Role) (types.Role, error) {
				if types.IsSystemResource(r) {
					require.Contains(t, expectedSystemRoles, r.GetName())
					return r, nil
				}
				require.Equal(t, teleport.PresetEditorRoleName, r.GetName())
				return r, nil
			})

		err = createPresetRoles(ctx, roleManager)
		require.NoError(t, err)
		require.Empty(t, remainingPresets)
		roleManager.AssertExpectations(t)
	}

	t.Run("Does not upsert roles if nothing changes", func(t *testing.T) {
		upsertRoleTest(t, presetRoleNames, nil)
	})

	t.Run("Enterprise", func(t *testing.T) {
		modules.SetTestModules(t, &modules.TestModules{
			TestBuildType: modules.BuildEnterprise,
		})

		enterprisePresetRoleNames := append([]string{
			teleport.PresetGroupAccessRoleName,
			teleport.PresetRequesterRoleName,
			teleport.PresetReviewerRoleName,
			teleport.PresetDeviceAdminRoleName,
			teleport.PresetDeviceEnrollRoleName,
			teleport.PresetRequireTrustedDeviceRoleName,
			teleport.SystemOktaRequesterRoleName, // This is treated as a preset
		}, presetRoleNames...)

		enterpriseSystemRoleNames := []string{
			teleport.SystemAutomaticAccessApprovalRoleName,
			teleport.SystemOktaAccessRoleName,
			teleport.SystemIdentityCenterAccessRoleName,
		}

		enterpriseUsers := []types.User{
			services.NewSystemAutomaticAccessBotUser(),
		}

		t.Run("EmptyCluster", func(t *testing.T) {
			as := newTestAuthServer(ctx, t)
			clock := clockwork.NewFakeClock()
			as.SetClock(clock)

			// Run multiple times to simulate starting auth on an
			// existing cluster and asserting that everything still
			// returns success
			for i := 0; i < 2; i++ {
				err := createPresetRoles(ctx, as)
				require.NoError(t, err)

				err = createPresetUsers(ctx, as)
				require.NoError(t, err)
			}

			// Preset Roles were created
			for _, role := range append(enterprisePresetRoleNames, enterpriseSystemRoleNames...) {
				_, err := as.GetRole(ctx, role)
				require.NoError(t, err)
			}

			// Preset Users were created
			for _, user := range enterpriseUsers {
				_, err := as.GetUser(ctx, user.GetName(), false)
				require.NoError(t, err)
			}
		})

		t.Run("Does not upsert roles if nothing changes", func(t *testing.T) {
			upsertRoleTest(t, enterprisePresetRoleNames, enterpriseSystemRoleNames)
		})

		t.Run("System users are always upserted", func(t *testing.T) {
			ctx := context.Background()
			sysUser := services.NewSystemAutomaticAccessBotUser().(*types.UserV2)

			// GIVEN a user database...
			auth := newMockUserManager(t)

			// Set the expectation that all user creations will succeed EXCEPT
			// for our known system user
			auth.On("CreateUser", mock.Anything, mock.Anything).
				Run(requireSystemResource(t, 1)).
				Maybe().
				Return(sysUser, nil)

			// All attempts to upsert should succeed, and record the being upserted
			var upsertedUsers []string
			auth.On("UpsertUser", mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					u := args.Get(1).(types.User)
					upsertedUsers = append(upsertedUsers, u.GetName())
				}).
				Return(sysUser, nil)

			// WHEN I attempt to create the preset users...
			err := createPresetUsers(ctx, auth)

			// EXPECT that the process succeeds and the system user was upserted
			require.NoError(t, err)
			auth.AssertExpectations(t)
			require.Contains(t, upsertedUsers, sysUser.Metadata.Name)
		})
	})
}

func TestGetPresetUsers(t *testing.T) {
	// no preset users for OSS
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildOSS,
	})
	require.Empty(t, getPresetUsers())

	// preset user @teleport-access-approval-bot on enterprise
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
	})
	require.Equal(t, []types.User{
		services.NewSystemAutomaticAccessBotUser(),
	}, getPresetUsers())
}

type mockUserManager struct {
	mock.Mock
}

func newMockUserManager(t *testing.T) *mockUserManager {
	m := &mockUserManager{}
	m.Test(t)
	return m
}

func (m *mockUserManager) CreateUser(ctx context.Context, user types.User) (types.User, error) {
	type delegateFn = func(context.Context, types.User) (types.User, error)
	args := m.Called(ctx, user)
	if delegate, ok := args.Get(0).(delegateFn); ok {
		return delegate(ctx, user)
	}
	return args.Get(0).(types.User), args.Error(1)
}

func (m *mockUserManager) GetUser(ctx context.Context, username string, withSecrets bool) (types.User, error) {
	type delegateFn = func(ctx context.Context, username string, withSecrets bool) (types.User, error)
	args := m.Called(ctx, username, withSecrets)
	if delegate, ok := args.Get(0).(delegateFn); ok {
		return delegate(ctx, username, withSecrets)
	}
	return args.Get(0).(types.User), args.Error(1)
}

func (m *mockUserManager) UpsertUser(ctx context.Context, user types.User) (types.User, error) {
	type delegateFn = func(context.Context, types.User) (types.User, error)
	args := m.Called(ctx, user)
	if delegate, ok := args.Get(0).(delegateFn); ok {
		return delegate(ctx, user)
	}
	return args.Get(0).(types.User), args.Error(1)
}

var _ PresetUsers = &mockUserManager{}

type mockRoleManager struct {
	mock.Mock
}

var _ PresetRoleManager = &mockRoleManager{}

func newMockRoleManager(t *testing.T) *mockRoleManager {
	m := &mockRoleManager{}
	m.Test(t)
	return m
}

// CreateRole creates a role.
func (m *mockRoleManager) CreateRole(ctx context.Context, role types.Role) (types.Role, error) {
	type delegateFn = func(context.Context, types.Role) (types.Role, error)
	args := m.Called(ctx, role)
	if delegate, ok := args[0].(delegateFn); ok {
		return delegate(ctx, role)
	}
	if args[0] == nil {
		return nil, args.Error(1)
	}
	return args[0].(types.Role), args.Error(1)
}

func (m *mockRoleManager) GetRole(ctx context.Context, name string) (types.Role, error) {
	type delegateFn = func(context.Context, string) (types.Role, error)
	args := m.Called(ctx, name)
	if delegate, ok := args[0].(delegateFn); ok {
		return delegate(ctx, name)
	}
	if args[0] == nil {
		return nil, args.Error(1)
	}
	return args[0].(types.Role), args.Error(1)
}

func (m *mockRoleManager) UpsertRole(ctx context.Context, role types.Role) (types.Role, error) {
	type delegateFn = func(context.Context, types.Role) (types.Role, error)
	args := m.Called(ctx, role)
	if delegate, ok := args[0].(delegateFn); ok {
		return delegate(ctx, role)
	}
	return args[0].(types.Role), args.Error(1)
}

func requireSystemResource(t *testing.T, argno int) func(mock.Arguments) {
	return func(args mock.Arguments) {
		argOfInterest := args[argno]
		require.Implements(t, (*types.Resource)(nil), argOfInterest)
		require.True(t, types.IsSystemResource(argOfInterest.(types.Resource)))
	}
}

func toSet(items []string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, v := range items {
		result[v] = struct{}{}
	}
	return result
}

func setupConfig(t *testing.T) InitConfig {
	tempDir := t.TempDir()

	bk, err := lite.New(context.TODO(), backend.Params{"path": tempDir})
	require.NoError(t, err)

	processStorage, err := storage.NewProcessStorage(
		context.Background(),
		filepath.Join(tempDir, teleport.ComponentProcess),
	)
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		bk.Close()
		processStorage.Close()
	})

	return InitConfig{
		DataDir:                 tempDir,
		HostUUID:                "00000000-0000-0000-0000-000000000000",
		NodeName:                "foo",
		Backend:                 bk,
		VersionStorage:          processStorage,
		Authority:               testauthority.New(),
		ClusterAuditConfig:      types.DefaultClusterAuditConfig(),
		ClusterNetworkingConfig: types.DefaultClusterNetworkingConfig(),
		SessionRecordingConfig:  types.DefaultSessionRecordingConfig(),
		ClusterName:             clusterName,
		StaticTokens:            types.DefaultStaticTokens(),
		AuthPreference:          types.DefaultAuthPreference(),
		SkipPeriodicOperations:  true,
		Tracer:                  tracing.NoopTracer(teleport.ComponentAuth),
	}
}

func newWebauthnAuthPreferenceConfigFromFile(t *testing.T) types.AuthPreference {
	ap, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorWebauthn,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	return ap
}

// Example resources generated using `tctl get all --with-secrets`.
const (
	hostCAYAML = `kind: cert_authority
metadata:
  id: 1630515580178991000
  name: me.localhost
spec:
  active_keys:
    ssh:
    - private_key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcFFJQkFBS0NBUUVBMGpYWHA5SmM3amFVVThydC9NUXZyZjY3TXZoZnk3ZG5ldHoyenBKWFFNNmJQUG5PCjdQVGdIV1o1TFFndVVJTldXLzN6ci9xa2V6UHhmU0lXaUVlcDlRTy9HSytBWHVNWTdmUThNTEc2cS93ckM3UU8KeWxaY3htWGZuS2ZXSkV5RUdralA5VlZmakd3UEdxRUNGWmZtT1pJdXFMVWVadEJKVzduZTZWbkYvWnYzbUhBZwpkc1FFRHphTjdlQnN2NHJyOVhQeHI4anh4Tzg5TjZZN0lsRStoUXNiT3FNSVVsSUVvSk0xYjBOcVFldlhHLzhVClBWWXJLK21hR2dNclNqZHppTjl6VHUzRzNCKzJ3SkJNZzd3VitMQjlJRUU0Y2NHYVVDbUM0QzBuZlByM283emgKTE1UNTZpdGZLdGw0TlkrN1NjRHVSSkN2TkFra3IrNkZtVUc5NXdJREFRQUJBb0lCQVFEUUVJTVlsVnR1WFkrTApNTDFIQjFpNk8veEdneGt1cHFaQ01od0ljMGp4Mkk1SFdHdThsdFNOeFRRRG9xbFUvK3FtdTBKTTJTV2MzTmtXCkpudHZBSi8wNkhScGxxelZQcXNhUERpbmFnTiszK1lyZTFsNFpPc0haU1prQkt3czJaK1g5S0lDRHpLMzV1MDgKU2ttcDNlUCs3L1pHL3A3TTNUVC9HWWJPS2hHUzV0VXdTY291NXV3aFMxQjFJOFNQQW05UGdsMmkyTjVib2s2YgpQTHBqT0xwOUZPVmpVMlNXYUhVa0ZiUUc2TENzOGlJdVdlVzdvMEtkY3B1QndXeGQxd0p3bm5nbjlJVzJ0aUdyCnZLTGlsYzJLR2xITnJxZldiRWw4Z2N5eXBKeUNqZEIvVXkvOFFBOG9uOHBnK1hldThrcnFyRjNTZm5iZHNOaDQKVmNKV09zTUJBb0dCQVBvY1E1ZWd0enZSL1djR0ZtbUxNbTUxTEVDMytPOWxVSUMwMGZ0Q1lkYW5vK0NGUTdEbgpQd0IyLzUyZWZMeUhLUlpjc0lrd2IreXZjYXp3eG9BWlNzMnRlV1FqRUJoOXdRbGlzWlNacVJIVEVMbnY0WjdvCkFMM3JmUjNtM3FHR0tmMm9UQnQ3dEVCcEMwZEVITDlxOVRpbkJJVDVKWGVSSVNzZUcyOGFKUG9IQW9HQkFOY3AKREtjc2hVKzJnaSs2V3p4eThOU3gyMkV5Y29FdUFKbWlSdm1jcExNNE9OTW03dUlQQ2RDYWYyY0UxYzJjVUg5OAppRGw2ZzBYN1ViUjQ4NzdJeVluMi9TU2hiOG56R3ZBNmVBbGJYZk9PYlYvTkIrczUxQzduTGFVSUE1ZEkyTE1WClRJL2xRTnV4cDhieXZDNjU3UVFQZk9jVXlGMVZIVVo5VzNPbU1hVWhBb0dCQU5RREZZRDQ2Wm81M1RaeHdKbmoKTnZMUFBKMzMxWHNKUlA1MVNQSldTUjF1cWNudTdYeU42YWY1TjZGaThaWFdkUXZSc292NGxVZnJTTTh5b3ZGLwpmeHR1aTlKSXJxSTBKMmhQVXYwR2JIMEJqOUl0OS9GOTlQTUpKZHd0RWxlVnBRNnlsU0ZPOFhNUUdGRm0rWCtCCnFURkcwdHZ0WHNkR0xQbWg0ZHVDTEFvTkFvR0FLSS9aamM2TDEwbzk0c2VNR2FwRmtxTnhDekxhZVZYMTBRRFIKeG83c1Vja2dsVlg2cE8xVzJWZTIrdkhqYUo2MllrSlU0QmtqbEZiYndWMG4vbWlWN2dkOUU2SEhsRmZiVlR5QQprcXNCM0QrV2lQLzdKVEpDdVJEbC92Mnl4NXQ1RnRIR0hENkk2cUhrVWxKQ2ZjQ1pXVEdlUjJZWW05Zkc3Qm9ICjJwYVROMkVDZ1lFQXRUQ2o1M253M213RDE1ZjAyZ2pKcWNjeHBkRk1lYlJpNUltMXJ5RUFaSzhwTmNlL2xNclgKVXM1KzM2eFpHWG1CR1ByMVl0UmxIVThQL1owWFdJM3ZDOFpLRHB6SnB4dnB6QmRFV0pUZGlPZEhpV2J4TzZOOQpiYjh3ZUM4STl4ZDlwd0cyMHdYNUVnR0krekdjMlNGa1VDTWROd1JkV3NSTWNCMklMeVV0MmZzPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
      public_key: c3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCQVFEU05kZW4wbHp1TnBSVHl1Mzh4Qyt0L3JzeStGL0x0MmQ2M1BiT2tsZEF6cHM4K2M3czlPQWRabmt0Q0M1UWcxWmIvZk92K3FSN00vRjlJaGFJUjZuMUE3OFlyNEJlNHhqdDlEd3dzYnFyL0NzTHRBN0tWbHpHWmQrY3A5WWtUSVFhU00vMVZWK01iQThhb1FJVmwrWTVraTZvdFI1bTBFbGJ1ZDdwV2NYOW0vZVljQ0IyeEFRUE5vM3Q0R3kvaXV2MWMvR3Z5UEhFN3owM3Bqc2lVVDZGQ3hzNm93aFNVZ1Nna3pWdlEycEI2OWNiL3hROVZpc3I2Wm9hQXl0S04zT0kzM05PN2NiY0g3YkFrRXlEdkJYNHNIMGdRVGh4d1pwUUtZTGdMU2Q4K3ZlanZPRXN4UG5xSzE4cTJYZzFqN3RKd081RWtLODBDU1N2N29XWlFiM24K
    tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURlVENDQW1HZ0F3SUJBZ0lSQUlvWW1QRUhNV0wwZmp6VkZVZ2l2Ykl3RFFZSktvWklodmNOQVFFTEJRQXcKVmpFUU1BNEdBMVVFQ2hNSGVtRnljWFZ2YmpFUU1BNEdBMVVFQXhNSGVtRnljWFZ2YmpFd01DNEdBMVVFQlJNbgpNVGd6TlRZeE1UZ3dOVFkxTXprMk1qUTNNRFV4TXpRM05qa3pOREV5TmpNME56Y3dPRFkyTUI0WERUSXhNRGt3Ck1URTJOVGswTUZvWERUTXhNRGd6TURFMk5UazBNRm93VmpFUU1BNEdBMVVFQ2hNSGVtRnljWFZ2YmpFUU1BNEcKQTFVRUF4TUhlbUZ5Y1hWdmJqRXdNQzRHQTFVRUJSTW5NVGd6TlRZeE1UZ3dOVFkxTXprMk1qUTNNRFV4TXpRMwpOamt6TkRFeU5qTTBOemN3T0RZMk1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBTUlJQkNnS0NBUUVBCjlzNGgrUTlpKzBlQ0c0VWNtSThFbEg1MHBlWmFkV2JzdzZDUStCZmF0em1NQWJEa016WGpSTHlNZy8xNFdSRGsKWTU4OUpCWVgrTzRBYXBRNno5MFRhTzhHamN1RmxvcWlFcTZOci9VTjJMYnc2am9KdTYvQ0dmcWIzNXZKT1NDdApKMWp5am5Cc2ZVNTRzRGFGcWpPRG1BL2l3YjNsSlZSV1pmYUdQSGZtRTRUcHJCSzdXV2FHbmlDZktUdGQza3lHCnkzWXpGZEJzSDU0OU9Lc1BFUlJOdTVCdlpzcmZWazRDdnlKNWVxREE1NlBaU1pmSVptZEk4VUdsRmk0V3lhMHEKUHRtWmg4bURGSnpVNnNXZWY5bTRqM3grMmF1UFEra2M3cjJneFdLZ2lTR1FBT2hLU0VhaEZDUW9UU3NzemVReApoZG9xamZUK1NsUWlQQlhJbDhTbzlRSURBUUFCbzBJd1FEQU9CZ05WSFE4QkFmOEVCQU1DQXFRd0R3WURWUjBUCkFRSC9CQVV3QXdFQi96QWRCZ05WSFE0RUZnUVVlSDJQTUhjRmwwTnkvQmozcE4xTExlSStPZTB3RFFZSktvWkkKaHZjTkFRRUxCUUFEZ2dFQkFMeU5aRXByOWtWbk5ORERiWWxWd0M1bnRlUEU2Z0ZQdGRjU3JjYmcyU01temtMMgppSjhCQnJ1ZVBmTllrMVE0c2xlN0FDeTRQL1dzU2xYZThqQkY0bm5KaUkxM2kvaUtQOEJzNVVIQWwwUTJ6RnlMCmp0Sk9JRlFuT1hiUmxQU1RQY21jcWdVUUcrL3lGOGhQVndHNnRwOVZoeXlCVkdlMDRlWU44eVNnbkZKTUloNGcKeDEzcytTWjE1RFkzSERnTGNCSjA5dXA4Q3NJdGV1RG05aG1jbGJ1bUc4OVEyQ3I4T25TcmYzTDNFSVhyelZBbwp3eEUwN21IWnkxSjBvNnljdWJXOWVuS2J3bTRKRHRhaHc4QzlERitvQXJlUEdaSXJXdVZpWFZYeTV6S2NOQW9LCjVXY0Q5RHdjcTFFUnVreGNFWWlrV05BV1FFNnVaMGVYZ0pKWlJ2OD0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
      key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBOXM0aCtROWkrMGVDRzRVY21JOEVsSDUwcGVaYWRXYnN3NkNRK0JmYXR6bU1BYkRrCk16WGpSTHlNZy8xNFdSRGtZNTg5SkJZWCtPNEFhcFE2ejkwVGFPOEdqY3VGbG9xaUVxNk5yL1VOMkxidzZqb0oKdTYvQ0dmcWIzNXZKT1NDdEoxanlqbkJzZlU1NHNEYUZxak9EbUEvaXdiM2xKVlJXWmZhR1BIZm1FNFRwckJLNwpXV2FHbmlDZktUdGQza3lHeTNZekZkQnNINTQ5T0tzUEVSUk51NUJ2WnNyZlZrNEN2eUo1ZXFEQTU2UFpTWmZJClptZEk4VUdsRmk0V3lhMHFQdG1aaDhtREZKelU2c1dlZjltNGozeCsyYXVQUStrYzdyMmd4V0tnaVNHUUFPaEsKU0VhaEZDUW9UU3NzemVReGhkb3FqZlQrU2xRaVBCWElsOFNvOVFJREFRQUJBb0lCQUhkODZ1TzYrRS94bWVNYQorZkkrWTVoRTlOS1JDTUNJT1I2cE1TWjczZzhSRkdDSk5LSTZkN0tDbW9FWWlWaU5uaFZCTmdldmpxR2RFS1NJCjZVUlRveDhOZ2gzS0ovM3ZWbkkzQWkvck0yMzFmQVBhWDNYM3JNQ0pIVWdRRTBiT05DYTFvSkVuaXM3TDNCQnMKQlNDVzJpSVhwcy9uMFBYV3RCR2ZYZlFPbEZ4ajdKbXdwdDlqYTYyankxa3hlTVdMemlqQUZ1QzRwRTl3WUVaVApGa0ovb2FVdzR4WXM5N0pnelRNUHFJSlUzMStNYTNoNGFPQ2lPK2RzSExhK2tRUkZIN0tHOUtRaWlRYzdrTmdGCnljN2s3L2s3UVJSbG1mZmZLN3E3YUtVNkNueHVxOG05UThob2Q5QVdjQ3ZyTm9jSmVjSzcvSzFzNlhMYXhSTFgKRGFLc0xRMENnWUVBK0lBOGNRVmcveU5xbGQ5c2lHZ0NETy9DVUNwM21uZjdxZTNQeEZIWHVSdFNzaHJheE9qVQpRVXFtbHV5VWRCZTZLQnA4ZWxoVmFlYlZsdFNNd2xyWXNpUlIxaTV0SDNnRGUybXRheE5VZ0xtTGoyZUhVWGN3Cmk1UW1yQ1AyTzY4V0xQZjg4SkkwQWtOWWx3em5aWUg2VEZKbFp0MkxoU3ZRTG1qeFArWDZTT01DZ1lFQS9rREwKNndJMjVoYU1DbTBSWitrQWM1T2FJcUx0OUNWeWNTaGdzblBjK2xBQll0ZVRkclhjVW1FaklQYW1uZ1pKZUpvSgpoUDNxaHBja1JVcWwrSldhN3lXRnJocTdyTHlpTVNBb0VwaXdQMkRUMkxROWFXR0lONjdwSU9ObWtpamU2NmNNCjBsd05hQTBtL3Q5ZWZUZ2RLOUpHYnNoaTVnRFFiTFVtMW5Gd1prY0NnWUFMenJ3UWVycnpKSkdwOFdYTXpYUmIKZlFEMG9pL3dyUWJPT2ppSEVZUjRqUzNPdkt2c2MwdXlsb04zNUdIaGFrYzBKSjRKaWl6MHpUMFUzNkNZazR4OApXbkZ4QmQrMWdSUlpSdG93bmtpRG5VMWVVUU1EQWZEU2tRV05aR0FNMGZMeHpBNit0NU8xRDlJanl6OHJlWk9WCkVNMDBxQTQ3RTZ2ZXFLbmQ2V1dORlFLQmdBa3Z6aTV2cGd3cVJHVWNDOFQxWms3R3hvcjUyQjg2T3loYmpTTGwKak5aK2pZNUV1ODlPUXVlM0dzM1dHNjhhQ3cyUWcwZUs1UzUzeDVlNVdzWGdvZmlDSXBKbjVPQVk4TU5WcGgwRgo1MWhpNTBTdFBvclFPMXZIdGlTNkVycTFQMWpFY0hJcFlWS2hKd2VPaXB0N3E1SXB4dUc1MjlqenJwUSs5MmhJCk1RZUJBb0dCQUkrNnE2OGFvdW41bE5sU3NoTVg0citoZlNtbFhXYmcvdU1OQXROdWhtZndmSE0zWmR3SU9ZWlYKRTloalFNQXh6RFduZkx1ajljRkJQM2NER0s0NytoU0U2N082eHBrYk9BYzk4dUlXZVEzb3lyWEd5akQ3TGdrYwpxajFRa2ZiZE4wMU1BdEN0MDhaSlRwZzR1OUdXUUVVQWY2ckJaKzJPN2ZVdEJuUG40TXpsCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
  additional_trusted_keys: {}
  checking_keys:
  - c3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCQVFEU05kZW4wbHp1TnBSVHl1Mzh4Qyt0L3JzeStGL0x0MmQ2M1BiT2tsZEF6cHM4K2M3czlPQWRabmt0Q0M1UWcxWmIvZk92K3FSN00vRjlJaGFJUjZuMUE3OFlyNEJlNHhqdDlEd3dzYnFyL0NzTHRBN0tWbHpHWmQrY3A5WWtUSVFhU00vMVZWK01iQThhb1FJVmwrWTVraTZvdFI1bTBFbGJ1ZDdwV2NYOW0vZVljQ0IyeEFRUE5vM3Q0R3kvaXV2MWMvR3Z5UEhFN3owM3Bqc2lVVDZGQ3hzNm93aFNVZ1Nna3pWdlEycEI2OWNiL3hROVZpc3I2Wm9hQXl0S04zT0kzM05PN2NiY0g3YkFrRXlEdkJYNHNIMGdRVGh4d1pwUUtZTGdMU2Q4K3ZlanZPRXN4UG5xSzE4cTJYZzFqN3RKd081RWtLODBDU1N2N29XWlFiM24K
  cluster_name: me.localhost
  signing_alg: 3
  signing_keys:
  - LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcFFJQkFBS0NBUUVBMGpYWHA5SmM3amFVVThydC9NUXZyZjY3TXZoZnk3ZG5ldHoyenBKWFFNNmJQUG5PCjdQVGdIV1o1TFFndVVJTldXLzN6ci9xa2V6UHhmU0lXaUVlcDlRTy9HSytBWHVNWTdmUThNTEc2cS93ckM3UU8KeWxaY3htWGZuS2ZXSkV5RUdralA5VlZmakd3UEdxRUNGWmZtT1pJdXFMVWVadEJKVzduZTZWbkYvWnYzbUhBZwpkc1FFRHphTjdlQnN2NHJyOVhQeHI4anh4Tzg5TjZZN0lsRStoUXNiT3FNSVVsSUVvSk0xYjBOcVFldlhHLzhVClBWWXJLK21hR2dNclNqZHppTjl6VHUzRzNCKzJ3SkJNZzd3VitMQjlJRUU0Y2NHYVVDbUM0QzBuZlByM283emgKTE1UNTZpdGZLdGw0TlkrN1NjRHVSSkN2TkFra3IrNkZtVUc5NXdJREFRQUJBb0lCQVFEUUVJTVlsVnR1WFkrTApNTDFIQjFpNk8veEdneGt1cHFaQ01od0ljMGp4Mkk1SFdHdThsdFNOeFRRRG9xbFUvK3FtdTBKTTJTV2MzTmtXCkpudHZBSi8wNkhScGxxelZQcXNhUERpbmFnTiszK1lyZTFsNFpPc0haU1prQkt3czJaK1g5S0lDRHpLMzV1MDgKU2ttcDNlUCs3L1pHL3A3TTNUVC9HWWJPS2hHUzV0VXdTY291NXV3aFMxQjFJOFNQQW05UGdsMmkyTjVib2s2YgpQTHBqT0xwOUZPVmpVMlNXYUhVa0ZiUUc2TENzOGlJdVdlVzdvMEtkY3B1QndXeGQxd0p3bm5nbjlJVzJ0aUdyCnZLTGlsYzJLR2xITnJxZldiRWw4Z2N5eXBKeUNqZEIvVXkvOFFBOG9uOHBnK1hldThrcnFyRjNTZm5iZHNOaDQKVmNKV09zTUJBb0dCQVBvY1E1ZWd0enZSL1djR0ZtbUxNbTUxTEVDMytPOWxVSUMwMGZ0Q1lkYW5vK0NGUTdEbgpQd0IyLzUyZWZMeUhLUlpjc0lrd2IreXZjYXp3eG9BWlNzMnRlV1FqRUJoOXdRbGlzWlNacVJIVEVMbnY0WjdvCkFMM3JmUjNtM3FHR0tmMm9UQnQ3dEVCcEMwZEVITDlxOVRpbkJJVDVKWGVSSVNzZUcyOGFKUG9IQW9HQkFOY3AKREtjc2hVKzJnaSs2V3p4eThOU3gyMkV5Y29FdUFKbWlSdm1jcExNNE9OTW03dUlQQ2RDYWYyY0UxYzJjVUg5OAppRGw2ZzBYN1ViUjQ4NzdJeVluMi9TU2hiOG56R3ZBNmVBbGJYZk9PYlYvTkIrczUxQzduTGFVSUE1ZEkyTE1WClRJL2xRTnV4cDhieXZDNjU3UVFQZk9jVXlGMVZIVVo5VzNPbU1hVWhBb0dCQU5RREZZRDQ2Wm81M1RaeHdKbmoKTnZMUFBKMzMxWHNKUlA1MVNQSldTUjF1cWNudTdYeU42YWY1TjZGaThaWFdkUXZSc292NGxVZnJTTTh5b3ZGLwpmeHR1aTlKSXJxSTBKMmhQVXYwR2JIMEJqOUl0OS9GOTlQTUpKZHd0RWxlVnBRNnlsU0ZPOFhNUUdGRm0rWCtCCnFURkcwdHZ0WHNkR0xQbWg0ZHVDTEFvTkFvR0FLSS9aamM2TDEwbzk0c2VNR2FwRmtxTnhDekxhZVZYMTBRRFIKeG83c1Vja2dsVlg2cE8xVzJWZTIrdkhqYUo2MllrSlU0QmtqbEZiYndWMG4vbWlWN2dkOUU2SEhsRmZiVlR5QQprcXNCM0QrV2lQLzdKVEpDdVJEbC92Mnl4NXQ1RnRIR0hENkk2cUhrVWxKQ2ZjQ1pXVEdlUjJZWW05Zkc3Qm9ICjJwYVROMkVDZ1lFQXRUQ2o1M253M213RDE1ZjAyZ2pKcWNjeHBkRk1lYlJpNUltMXJ5RUFaSzhwTmNlL2xNclgKVXM1KzM2eFpHWG1CR1ByMVl0UmxIVThQL1owWFdJM3ZDOFpLRHB6SnB4dnB6QmRFV0pUZGlPZEhpV2J4TzZOOQpiYjh3ZUM4STl4ZDlwd0cyMHdYNUVnR0krekdjMlNGa1VDTWROd1JkV3NSTWNCMklMeVV0MmZzPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
  tls_key_pairs:
  - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURlVENDQW1HZ0F3SUJBZ0lSQUlvWW1QRUhNV0wwZmp6VkZVZ2l2Ykl3RFFZSktvWklodmNOQVFFTEJRQXcKVmpFUU1BNEdBMVVFQ2hNSGVtRnljWFZ2YmpFUU1BNEdBMVVFQXhNSGVtRnljWFZ2YmpFd01DNEdBMVVFQlJNbgpNVGd6TlRZeE1UZ3dOVFkxTXprMk1qUTNNRFV4TXpRM05qa3pOREV5TmpNME56Y3dPRFkyTUI0WERUSXhNRGt3Ck1URTJOVGswTUZvWERUTXhNRGd6TURFMk5UazBNRm93VmpFUU1BNEdBMVVFQ2hNSGVtRnljWFZ2YmpFUU1BNEcKQTFVRUF4TUhlbUZ5Y1hWdmJqRXdNQzRHQTFVRUJSTW5NVGd6TlRZeE1UZ3dOVFkxTXprMk1qUTNNRFV4TXpRMwpOamt6TkRFeU5qTTBOemN3T0RZMk1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBTUlJQkNnS0NBUUVBCjlzNGgrUTlpKzBlQ0c0VWNtSThFbEg1MHBlWmFkV2JzdzZDUStCZmF0em1NQWJEa016WGpSTHlNZy8xNFdSRGsKWTU4OUpCWVgrTzRBYXBRNno5MFRhTzhHamN1RmxvcWlFcTZOci9VTjJMYnc2am9KdTYvQ0dmcWIzNXZKT1NDdApKMWp5am5Cc2ZVNTRzRGFGcWpPRG1BL2l3YjNsSlZSV1pmYUdQSGZtRTRUcHJCSzdXV2FHbmlDZktUdGQza3lHCnkzWXpGZEJzSDU0OU9Lc1BFUlJOdTVCdlpzcmZWazRDdnlKNWVxREE1NlBaU1pmSVptZEk4VUdsRmk0V3lhMHEKUHRtWmg4bURGSnpVNnNXZWY5bTRqM3grMmF1UFEra2M3cjJneFdLZ2lTR1FBT2hLU0VhaEZDUW9UU3NzemVReApoZG9xamZUK1NsUWlQQlhJbDhTbzlRSURBUUFCbzBJd1FEQU9CZ05WSFE4QkFmOEVCQU1DQXFRd0R3WURWUjBUCkFRSC9CQVV3QXdFQi96QWRCZ05WSFE0RUZnUVVlSDJQTUhjRmwwTnkvQmozcE4xTExlSStPZTB3RFFZSktvWkkKaHZjTkFRRUxCUUFEZ2dFQkFMeU5aRXByOWtWbk5ORERiWWxWd0M1bnRlUEU2Z0ZQdGRjU3JjYmcyU01temtMMgppSjhCQnJ1ZVBmTllrMVE0c2xlN0FDeTRQL1dzU2xYZThqQkY0bm5KaUkxM2kvaUtQOEJzNVVIQWwwUTJ6RnlMCmp0Sk9JRlFuT1hiUmxQU1RQY21jcWdVUUcrL3lGOGhQVndHNnRwOVZoeXlCVkdlMDRlWU44eVNnbkZKTUloNGcKeDEzcytTWjE1RFkzSERnTGNCSjA5dXA4Q3NJdGV1RG05aG1jbGJ1bUc4OVEyQ3I4T25TcmYzTDNFSVhyelZBbwp3eEUwN21IWnkxSjBvNnljdWJXOWVuS2J3bTRKRHRhaHc4QzlERitvQXJlUEdaSXJXdVZpWFZYeTV6S2NOQW9LCjVXY0Q5RHdjcTFFUnVreGNFWWlrV05BV1FFNnVaMGVYZ0pKWlJ2OD0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
    key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBOXM0aCtROWkrMGVDRzRVY21JOEVsSDUwcGVaYWRXYnN3NkNRK0JmYXR6bU1BYkRrCk16WGpSTHlNZy8xNFdSRGtZNTg5SkJZWCtPNEFhcFE2ejkwVGFPOEdqY3VGbG9xaUVxNk5yL1VOMkxidzZqb0oKdTYvQ0dmcWIzNXZKT1NDdEoxanlqbkJzZlU1NHNEYUZxak9EbUEvaXdiM2xKVlJXWmZhR1BIZm1FNFRwckJLNwpXV2FHbmlDZktUdGQza3lHeTNZekZkQnNINTQ5T0tzUEVSUk51NUJ2WnNyZlZrNEN2eUo1ZXFEQTU2UFpTWmZJClptZEk4VUdsRmk0V3lhMHFQdG1aaDhtREZKelU2c1dlZjltNGozeCsyYXVQUStrYzdyMmd4V0tnaVNHUUFPaEsKU0VhaEZDUW9UU3NzemVReGhkb3FqZlQrU2xRaVBCWElsOFNvOVFJREFRQUJBb0lCQUhkODZ1TzYrRS94bWVNYQorZkkrWTVoRTlOS1JDTUNJT1I2cE1TWjczZzhSRkdDSk5LSTZkN0tDbW9FWWlWaU5uaFZCTmdldmpxR2RFS1NJCjZVUlRveDhOZ2gzS0ovM3ZWbkkzQWkvck0yMzFmQVBhWDNYM3JNQ0pIVWdRRTBiT05DYTFvSkVuaXM3TDNCQnMKQlNDVzJpSVhwcy9uMFBYV3RCR2ZYZlFPbEZ4ajdKbXdwdDlqYTYyankxa3hlTVdMemlqQUZ1QzRwRTl3WUVaVApGa0ovb2FVdzR4WXM5N0pnelRNUHFJSlUzMStNYTNoNGFPQ2lPK2RzSExhK2tRUkZIN0tHOUtRaWlRYzdrTmdGCnljN2s3L2s3UVJSbG1mZmZLN3E3YUtVNkNueHVxOG05UThob2Q5QVdjQ3ZyTm9jSmVjSzcvSzFzNlhMYXhSTFgKRGFLc0xRMENnWUVBK0lBOGNRVmcveU5xbGQ5c2lHZ0NETy9DVUNwM21uZjdxZTNQeEZIWHVSdFNzaHJheE9qVQpRVXFtbHV5VWRCZTZLQnA4ZWxoVmFlYlZsdFNNd2xyWXNpUlIxaTV0SDNnRGUybXRheE5VZ0xtTGoyZUhVWGN3Cmk1UW1yQ1AyTzY4V0xQZjg4SkkwQWtOWWx3em5aWUg2VEZKbFp0MkxoU3ZRTG1qeFArWDZTT01DZ1lFQS9rREwKNndJMjVoYU1DbTBSWitrQWM1T2FJcUx0OUNWeWNTaGdzblBjK2xBQll0ZVRkclhjVW1FaklQYW1uZ1pKZUpvSgpoUDNxaHBja1JVcWwrSldhN3lXRnJocTdyTHlpTVNBb0VwaXdQMkRUMkxROWFXR0lONjdwSU9ObWtpamU2NmNNCjBsd05hQTBtL3Q5ZWZUZ2RLOUpHYnNoaTVnRFFiTFVtMW5Gd1prY0NnWUFMenJ3UWVycnpKSkdwOFdYTXpYUmIKZlFEMG9pL3dyUWJPT2ppSEVZUjRqUzNPdkt2c2MwdXlsb04zNUdIaGFrYzBKSjRKaWl6MHpUMFUzNkNZazR4OApXbkZ4QmQrMWdSUlpSdG93bmtpRG5VMWVVUU1EQWZEU2tRV05aR0FNMGZMeHpBNit0NU8xRDlJanl6OHJlWk9WCkVNMDBxQTQ3RTZ2ZXFLbmQ2V1dORlFLQmdBa3Z6aTV2cGd3cVJHVWNDOFQxWms3R3hvcjUyQjg2T3loYmpTTGwKak5aK2pZNUV1ODlPUXVlM0dzM1dHNjhhQ3cyUWcwZUs1UzUzeDVlNVdzWGdvZmlDSXBKbjVPQVk4TU5WcGgwRgo1MWhpNTBTdFBvclFPMXZIdGlTNkVycTFQMWpFY0hJcFlWS2hKd2VPaXB0N3E1SXB4dUc1MjlqenJwUSs5MmhJCk1RZUJBb0dCQUkrNnE2OGFvdW41bE5sU3NoTVg0citoZlNtbFhXYmcvdU1OQXROdWhtZndmSE0zWmR3SU9ZWlYKRTloalFNQXh6RFduZkx1ajljRkJQM2NER0s0NytoU0U2N082eHBrYk9BYzk4dUlXZVEzb3lyWEd5akQ3TGdrYwpxajFRa2ZiZE4wMU1BdEN0MDhaSlRwZzR1OUdXUUVVQWY2ckJaKzJPN2ZVdEJuUG40TXpsCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
  type: host
sub_kind: host
version: v2`
	userCAYAML = `kind: cert_authority
metadata:
  id: 1630515579836524000
  name: me.localhost
spec:
  active_keys:
    ssh:
    - private_key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBMnZ1ekRQRHZWTElYZTdicDZIc1ZiM3NHak0xZ3lFa2hkWVBUMVVzZ01Kd1JuL1VSCk5EQTZST2pya2NUSEhyRXhnL002dVlQaGN2K2UweUFOZk9JMXROWVgvQ3hGcWdCcGFpRE5YRGpRajdyV2E4RDQKWmNyNkdIVXoza3VDN0k0QTJaUlJXK2M3ajhlZDFEbTlzZ0lpOW1BU3NXL0M2RTZTblEvVGlXVk9hTEdqdEVTdwo5L0lpenc1S2pPYnJLMGRXUXZiMXY1SW8rTUpVbko1NE5jWUlMRm9pYk9WeDdQU1FzWCtDK1JwQ3BIbFIrWnNWClVmSzFrd0l4eXg1Q25BTEMzcnlkOUFaSnY2aVdoci9uczZyWmU5b21xMHd6VUpnT045VE5aQjdZTjRhUmlYSmMKNkpsZmpjdUpnZ2ZDbVZaSWNyRm9wd3kzdnp4LzBxNVVCOVRNRFFJREFRQUJBb0lCQUhVNHkyNGdBMTJwUDl6ZgoyM0t4Z0pYK20xRUFGOURmSk9RTlAzWXNFdjB5YmxUY0VPdUk3WWc1enZCbkQ5Z2tMa2RlQ28rSVEwVVdCT1VyCmdVemFvcms4NmZYNWxRa2QwMUFXWXhmODZkZ213ZVZJbFMrWWFpeHhnT1I4TTRlQnRIN0VZSkQ3eE95QWhNSTQKYm8wOWk0MnJmQll6cDNoSHAwQWdXckp2NG5zenJtVGl2d2s4b0JRS2pGbDZ5VjU2L0g4K3VxaWlSS28vRDlPTApXSU51M1JXRTd4RVpJdlNBZlFMYUFyVG1XaXVVMkhrTklTSmJqRGxrd1phMmtjQ1VQUnVkbFJsOWw2WVM0ZUpSClhNU3RaVmQrcVRpVHhFc29TcTBjRmI3dTlYcEs0a3U3TXR5dDFHY0V0MnIvQzRrcXJFNFAyK3c3RmkvalN2VjYKdUZoWW41a0NnWUVBM3VuK0JMN21hSUhsOGxNclVrcG5SUFJNenllOGFyREplbnM4RXYzTUNyVkxLR0p5NUV6cwppc2ZmVythYTJSejhGOFk0U09aZjZaTk9CS3RzOHFYbkVxL0RhVEgyY3NxMFZXUWxNOFZQNkl3SHpTemNXaThwCnVVRUFOTFg3aG9maTFmZERGNnE2cTFrcmJtdjREa085RGdnRVpxVUR6KzB1dWZhTFNWWG05TzhDZ1lFQSszeFoKTGRJdjVjbXNHWTZhL0xXVjgwRjZHMVMxYUVJVDlBdXNpTThFcnhybEZpbjRSL2dDcEMzSjJod0ZKYldzUjN0OApMVUgxdWxqdE82Mmk2enkzUUlJa1RFNDRveUo0V1RVSEpWQ0VEYXFvcldySklWWTZxMFBlWGs5VDVwa1dzRFpOCldGeDRKQzZhdDZDcEtSTEJPSHdoSm1mYU53K1daamVob251L1pzTUNnWUJpMGI2UFlnV0luTlZRYUxoU3diTW8KS1ZrSG1LajVieWZTU1dGblZlV25kWms4N08vYjc1SUpML1AvcktwR3g0ZW1EblNUTkxXZU9YUWpzODhYZnA2QwpkVEtlcHN5SE5QOWV2NGVTZk0wZzNUcjBKUWdHWHRRVFVSS0RTNDJXcFJUVkg4azVhN0ZYRnErZlF2UHpkdW9QCmwxUkVJTEVnOHhkOHp5UU9QYXVtTndLQmdRREprZXlrMW5DL3ZMcXRyV2k2bncwMmNjZmVlaklCQTkyY1lYTUUKSVBJL0s4NXN5bTBQdWxEYnFUdStEM0ZzdlVYOThaTWhiMW4yNStvV1NHRnFMVHN3Z0Y5NXJjU2x0UzVEU2thVQorUWt2THhlT0VDWndDdjV4WWErdFplWDQwY0dtc1krakFGTG5wVmNyVWFIa292eXVPb2dUa1hBTmEvZi9yQjFvCjc4a0ZJd0tCZ0Q1cHVRd3NodXRYdmNOOCs1bUZDc2d6YW5hckYxSllPdTE2MkhVU2ZPQ2k4aS9VRDNWSUpYQmwKbWpNQVN4elhRamFSa2dQZXNqTmtsTEoyekJsK3ZLaG1jb3NJelNOdzM5b1ZMcC9ObTBiRzlpZmdUdmlha1N6MwpHNFhLYUdNczdRQ3h4bjZsc2ptYTRxYlFlU3JwWmVSMXVtOXJ4TVhLaVdGSUtERXN5TUpNCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
      public_key: c3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCQVFEYSs3TU04TzlVc2hkN3R1bm9leFZ2ZXdhTXpXRElTU0YxZzlQVlN5QXduQkdmOVJFME1EcEU2T3VSeE1jZXNUR0Q4enE1ZytGeS81N1RJQTE4NGpXMDFoZjhMRVdxQUdscUlNMWNPTkNQdXRacndQaGx5dm9ZZFRQZVM0THNqZ0RabEZGYjV6dVB4NTNVT2IyeUFpTDJZQkt4YjhMb1RwS2REOU9KWlU1b3NhTzBSTEQzOGlMUERrcU01dXNyUjFaQzl2Vy9raWo0d2xTY25uZzF4Z2dzV2lKczVYSHM5SkN4ZjRMNUdrS2tlVkg1bXhWUjhyV1RBakhMSGtLY0FzTGV2SjMwQmttL3FKYUd2K2V6cXRsNzJpYXJURE5RbUE0MzFNMWtIdGczaHBHSmNsem9tVitOeTRtQ0I4S1pWa2h5c1dpbkRMZS9QSC9TcmxRSDFNd04K
    tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURlRENDQW1DZ0F3SUJBZ0lRVENnQUVvV3BBa2U4YWdHandmVmtkVEFOQmdrcWhraUc5dzBCQVFzRkFEQlcKTVJBd0RnWURWUVFLRXdkNllYSnhkVzl1TVJBd0RnWURWUVFERXdkNllYSnhkVzl1TVRBd0xnWURWUVFGRXljeApNREV5TWprd01qRXdNakUwTmpjM05UQXlOREF6TnpBNE5qQXpPVEV6TmpNd056Y3lNemN3SGhjTk1qRXdPVEF4Ck1UWTFPVE01V2hjTk16RXdPRE13TVRZMU9UTTVXakJXTVJBd0RnWURWUVFLRXdkNllYSnhkVzl1TVJBd0RnWUQKVlFRREV3ZDZZWEp4ZFc5dU1UQXdMZ1lEVlFRRkV5Y3hNREV5TWprd01qRXdNakUwTmpjM05UQXlOREF6TnpBNApOakF6T1RFek5qTXdOemN5TXpjd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM5Cjg5dFE1YTB0bzQ5dkN3dmxFTUpYTnBiYkpzUVIram96QzJScFFTLzZBSHlHbytueXRKUTZSclp6UWlZeTVkb2UKUTArUmhPTGM0R1Q4R0JQdTBrRmUwejk4Zlp4TC8wb2kvZ0trVTgrM0VOaDFuN1krbjV0RVBvbHZsZTBZUm5lWAp3NWpna1lDZG1TTjFMZXVqYW96OUpyYXVUMzRPSGwzdFNoZ3pBUkV1ZlBVVDJQNXlaQXNCdW9nYzBHd055MUo4Cml1UXpDb0tNektOM2ZidFhYU2llU3pubWxQeWMyeXpIcW1UZGdQNnVDNWFrVXpnT3E5ZGFjUkpxVWJRVXlXOUUKVk80MWRQK2wrMXdzNmdSaFNuK29Sc0JLOG1TVW1JNjc4VjE2b3RxT1I2ZDl5VnZKVTZSbjEvMWIzYk5ES3I2awpYSWwwR1FYczNtajd0QUEzVHNCRkFnTUJBQUdqUWpCQU1BNEdBMVVkRHdFQi93UUVBd0lDcERBUEJnTlZIUk1CCkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCVFovZGxCRDdncWpKTU9rR2laSitGL0NXbkh5akFOQmdrcWhraUcKOXcwQkFRc0ZBQU9DQVFFQXU4cVZUZTYzZnd4UkM3cFU2bzRscldSby9ldnB3VkhkV04yT1NES0t4UU5ISURhbQovRDlnamhtdnphQWFKY3E2UmdkaTNCWlRuZDhsSDYzUlZuVHk4RzljUTNTYmFBMHlSQzMyQnQ3bjh6aUYxTTNqCkFxck5zV0hIWkZydkJKWkx2UDBqbms5RmRtSmFYc3o0cGgyRndYQjFTUkdFT3lVcFAvSS9OeVUyNFovRGFvOG4KWlRSazAvbHBVUis2SGp0UGliUTdJejkxcWtxNmsxcEY0SXdlbG5xU1Vqc2FjLzBGa1lib1I0UHkvT1BUbmtIRgpVR2doRUFHRzNnZXFsaGI4Q3FUYmRwKzhLb0R6RzNBOUtRUURBWUlMNkh5Mm9nVEJmU241Uk5WY2pLdHI0QS9UClhnSnlUdjdHZ0FabkZHT1dMc3B2TDhJQVdGYmN4T2hwV2hEaDNRPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
      key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBdmZQYlVPV3RMYU9QYndzTDVSRENWemFXMnliRUVmbzZNd3RrYVVFditnQjhocVBwCjhyU1VPa2EyYzBJbU11WGFIa05Qa1lUaTNPQmsvQmdUN3RKQlh0TS9mSDJjUy85S0l2NENwRlBQdHhEWWRaKzIKUHArYlJENkpiNVh0R0VaM2w4T1k0SkdBblpramRTM3JvMnFNL1NhMnJrOStEaDVkN1VvWU13RVJMbnoxRTlqKwpjbVFMQWJxSUhOQnNEY3RTZklya013cUNqTXlqZDMyN1YxMG9ua3M1NXBUOG5Oc3N4NnBrM1lEK3JndVdwRk00CkRxdlhXbkVTYWxHMEZNbHZSRlR1TlhUL3BmdGNMT29FWVVwL3FFYkFTdkprbEppT3UvRmRlcUxhamtlbmZjbGIKeVZPa1o5ZjlXOTJ6UXlxK3BGeUpkQmtGN041bys3UUFOMDdBUlFJREFRQUJBb0lCQVFDWkFiTHBxUGdrU1JtaQpncTFrS0duQ3dwQWxtMFpZak0wUWpONm5BZ0ZaU2NjRTFVZi9Yb0gvcHpJVUNYYW5qUXB6VWhqbnlMak0zbHU1CnpOTlJqajlsMkpmTStZbEtsaXJyb053VDdnYmxHVWFqQ0xGT0pGWjNWRUIwaDduaDBmRkhhQ0RlMDVWY1hSeDQKcVRLa0FaSHI0S0ZLSzNJSWdXRjdZREc1OCtRWkl0N01BdDQyc1NrUlhaenNyaUVzVWxZNU9xT1dlVHlkYy93dQpDK292V1FlOUU2bnRSc0EvY25nS0M1bnp6T0pCano3SlVTMGQ2UnBGK1Zodk9SRlZ6YTZKMnpRRXJsWWtFdVVUCkJYdThsVmhhcDVOU09nd3ByZXVmb2tNZVY4eGRNdUxuVEthdHVjajIwT3JFRUlHZVFrb08yTzhTeDJtNitOS0kKbzBTT2kzU0pBb0dCQU5LVkk4R0JHQUpSUlV2NVBnVkNDQ1VHSkFBRmtxY0FMTFlaZ0NMZCtLYStBVW41L2VONwpWemR4b2VqNG1UcnFOb0RLTnk5b0N2Z1RBWTlWZ3RZZllpdlNQbW5ZZk9FbTJUR2pLRTJSYzhBd0R5UWdHWFJVCm5WOUVueHRUUHF5dU9tQTFqa0ladXBLeUU0b0drdTE4MDRVaTB2d2pJUWI3cDdML0ZGYksvaGlmQW9HQkFPYnIKclBrOHdicnBlVk9QWmVudzVzbjVhTUpjdDM3emt4dHZzejVSVWlsZnZtVkdja1lyMU41SGpmZjVVVGcwNmNWdgp1N0dGY0NLQmhncFdqSXM3dHZ4cDhUVFM1VEZaWHZqY2VhRC91ZkFiN3ZUVnR0L1hRMFUwQ28rcDdzMENKWlZKCjY2MUtUU1RCYlFtR0xYYmNqNmFYdVM3bzFJM1FRU1l4NW5LV1F5aWJBb0dCQUtBTWZocUtGVWRkb1g5MnRhNmwKV3k5WWxXLzJ6RmxsQnBaNGx5em83QjArK0JmVGl5V2tEc3V5NzgzemMvS1ZKRXVLWlpzQVJxWDVQQXhHZjZSaQpRZWp3YUVObUtMT3ZKUkJXNDBEaE5jcHlQRy9HZmRJdXBWVk5BR2h5UW9aWC9VSTJNaU1IRHdpRGs5b3AyTzNyCkc1QnF3VlNsRm1zS1JaRUQwZCtOZE1ZZEFvR0FkVDNQRXJQd1FHL3RzNmtvdTBBZVRRbWVVS0EyWWZSVkNpY0sKUUdlVmFZQTg4THAxcG43Mmt1eU5mZ3ROVzFZeUlwWDZHOFYrQzJicm9UQVVKMVRvTVB1eEJYclY5dHBEUitMWQp0ZzlnWGpJd2ZvcExVUmJBQnRESFUrMlpXdWp1SC8vcDhvKzQzeUo5czhvMkp4VVFzaXB5VVFqUmNqYjcvT0owCitGU21RR1VDZ1lBdU5XcFhrbVkzbGtRSEFpU3RlblhTT2Zqc0xsbm5XWFJoTDVBYTZqRVZRT3FnUldVQnZ2YWkKK1RQRTNUTHQ5MGwveTZOdjU0dDdrT1QvSlEvREU4WmFiMERlTjdBRzRwRjMwNkxpYlpZNmswc3M1UDNXME8vbAozekJzQ0lEY3BHOWw4bDZSNkUwdHN0Z0I1c25hOE4rRzA2V3Q1R0M0UitRSVd1YTVpUmtVTXc9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
  additional_trusted_keys: {}
  checking_keys:
  - c3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCQVFEYSs3TU04TzlVc2hkN3R1bm9leFZ2ZXdhTXpXRElTU0YxZzlQVlN5QXduQkdmOVJFME1EcEU2T3VSeE1jZXNUR0Q4enE1ZytGeS81N1RJQTE4NGpXMDFoZjhMRVdxQUdscUlNMWNPTkNQdXRacndQaGx5dm9ZZFRQZVM0THNqZ0RabEZGYjV6dVB4NTNVT2IyeUFpTDJZQkt4YjhMb1RwS2REOU9KWlU1b3NhTzBSTEQzOGlMUERrcU01dXNyUjFaQzl2Vy9raWo0d2xTY25uZzF4Z2dzV2lKczVYSHM5SkN4ZjRMNUdrS2tlVkg1bXhWUjhyV1RBakhMSGtLY0FzTGV2SjMwQmttL3FKYUd2K2V6cXRsNzJpYXJURE5RbUE0MzFNMWtIdGczaHBHSmNsem9tVitOeTRtQ0I4S1pWa2h5c1dpbkRMZS9QSC9TcmxRSDFNd04K
  cluster_name: me.localhost
  signing_alg: 3
  signing_keys:
  - LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBMnZ1ekRQRHZWTElYZTdicDZIc1ZiM3NHak0xZ3lFa2hkWVBUMVVzZ01Kd1JuL1VSCk5EQTZST2pya2NUSEhyRXhnL002dVlQaGN2K2UweUFOZk9JMXROWVgvQ3hGcWdCcGFpRE5YRGpRajdyV2E4RDQKWmNyNkdIVXoza3VDN0k0QTJaUlJXK2M3ajhlZDFEbTlzZ0lpOW1BU3NXL0M2RTZTblEvVGlXVk9hTEdqdEVTdwo5L0lpenc1S2pPYnJLMGRXUXZiMXY1SW8rTUpVbko1NE5jWUlMRm9pYk9WeDdQU1FzWCtDK1JwQ3BIbFIrWnNWClVmSzFrd0l4eXg1Q25BTEMzcnlkOUFaSnY2aVdoci9uczZyWmU5b21xMHd6VUpnT045VE5aQjdZTjRhUmlYSmMKNkpsZmpjdUpnZ2ZDbVZaSWNyRm9wd3kzdnp4LzBxNVVCOVRNRFFJREFRQUJBb0lCQUhVNHkyNGdBMTJwUDl6ZgoyM0t4Z0pYK20xRUFGOURmSk9RTlAzWXNFdjB5YmxUY0VPdUk3WWc1enZCbkQ5Z2tMa2RlQ28rSVEwVVdCT1VyCmdVemFvcms4NmZYNWxRa2QwMUFXWXhmODZkZ213ZVZJbFMrWWFpeHhnT1I4TTRlQnRIN0VZSkQ3eE95QWhNSTQKYm8wOWk0MnJmQll6cDNoSHAwQWdXckp2NG5zenJtVGl2d2s4b0JRS2pGbDZ5VjU2L0g4K3VxaWlSS28vRDlPTApXSU51M1JXRTd4RVpJdlNBZlFMYUFyVG1XaXVVMkhrTklTSmJqRGxrd1phMmtjQ1VQUnVkbFJsOWw2WVM0ZUpSClhNU3RaVmQrcVRpVHhFc29TcTBjRmI3dTlYcEs0a3U3TXR5dDFHY0V0MnIvQzRrcXJFNFAyK3c3RmkvalN2VjYKdUZoWW41a0NnWUVBM3VuK0JMN21hSUhsOGxNclVrcG5SUFJNenllOGFyREplbnM4RXYzTUNyVkxLR0p5NUV6cwppc2ZmVythYTJSejhGOFk0U09aZjZaTk9CS3RzOHFYbkVxL0RhVEgyY3NxMFZXUWxNOFZQNkl3SHpTemNXaThwCnVVRUFOTFg3aG9maTFmZERGNnE2cTFrcmJtdjREa085RGdnRVpxVUR6KzB1dWZhTFNWWG05TzhDZ1lFQSszeFoKTGRJdjVjbXNHWTZhL0xXVjgwRjZHMVMxYUVJVDlBdXNpTThFcnhybEZpbjRSL2dDcEMzSjJod0ZKYldzUjN0OApMVUgxdWxqdE82Mmk2enkzUUlJa1RFNDRveUo0V1RVSEpWQ0VEYXFvcldySklWWTZxMFBlWGs5VDVwa1dzRFpOCldGeDRKQzZhdDZDcEtSTEJPSHdoSm1mYU53K1daamVob251L1pzTUNnWUJpMGI2UFlnV0luTlZRYUxoU3diTW8KS1ZrSG1LajVieWZTU1dGblZlV25kWms4N08vYjc1SUpML1AvcktwR3g0ZW1EblNUTkxXZU9YUWpzODhYZnA2QwpkVEtlcHN5SE5QOWV2NGVTZk0wZzNUcjBKUWdHWHRRVFVSS0RTNDJXcFJUVkg4azVhN0ZYRnErZlF2UHpkdW9QCmwxUkVJTEVnOHhkOHp5UU9QYXVtTndLQmdRREprZXlrMW5DL3ZMcXRyV2k2bncwMmNjZmVlaklCQTkyY1lYTUUKSVBJL0s4NXN5bTBQdWxEYnFUdStEM0ZzdlVYOThaTWhiMW4yNStvV1NHRnFMVHN3Z0Y5NXJjU2x0UzVEU2thVQorUWt2THhlT0VDWndDdjV4WWErdFplWDQwY0dtc1krakFGTG5wVmNyVWFIa292eXVPb2dUa1hBTmEvZi9yQjFvCjc4a0ZJd0tCZ0Q1cHVRd3NodXRYdmNOOCs1bUZDc2d6YW5hckYxSllPdTE2MkhVU2ZPQ2k4aS9VRDNWSUpYQmwKbWpNQVN4elhRamFSa2dQZXNqTmtsTEoyekJsK3ZLaG1jb3NJelNOdzM5b1ZMcC9ObTBiRzlpZmdUdmlha1N6MwpHNFhLYUdNczdRQ3h4bjZsc2ptYTRxYlFlU3JwWmVSMXVtOXJ4TVhLaVdGSUtERXN5TUpNCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
  tls_key_pairs:
  - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURlRENDQW1DZ0F3SUJBZ0lRVENnQUVvV3BBa2U4YWdHandmVmtkVEFOQmdrcWhraUc5dzBCQVFzRkFEQlcKTVJBd0RnWURWUVFLRXdkNllYSnhkVzl1TVJBd0RnWURWUVFERXdkNllYSnhkVzl1TVRBd0xnWURWUVFGRXljeApNREV5TWprd01qRXdNakUwTmpjM05UQXlOREF6TnpBNE5qQXpPVEV6TmpNd056Y3lNemN3SGhjTk1qRXdPVEF4Ck1UWTFPVE01V2hjTk16RXdPRE13TVRZMU9UTTVXakJXTVJBd0RnWURWUVFLRXdkNllYSnhkVzl1TVJBd0RnWUQKVlFRREV3ZDZZWEp4ZFc5dU1UQXdMZ1lEVlFRRkV5Y3hNREV5TWprd01qRXdNakUwTmpjM05UQXlOREF6TnpBNApOakF6T1RFek5qTXdOemN5TXpjd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM5Cjg5dFE1YTB0bzQ5dkN3dmxFTUpYTnBiYkpzUVIram96QzJScFFTLzZBSHlHbytueXRKUTZSclp6UWlZeTVkb2UKUTArUmhPTGM0R1Q4R0JQdTBrRmUwejk4Zlp4TC8wb2kvZ0trVTgrM0VOaDFuN1krbjV0RVBvbHZsZTBZUm5lWAp3NWpna1lDZG1TTjFMZXVqYW96OUpyYXVUMzRPSGwzdFNoZ3pBUkV1ZlBVVDJQNXlaQXNCdW9nYzBHd055MUo4Cml1UXpDb0tNektOM2ZidFhYU2llU3pubWxQeWMyeXpIcW1UZGdQNnVDNWFrVXpnT3E5ZGFjUkpxVWJRVXlXOUUKVk80MWRQK2wrMXdzNmdSaFNuK29Sc0JLOG1TVW1JNjc4VjE2b3RxT1I2ZDl5VnZKVTZSbjEvMWIzYk5ES3I2awpYSWwwR1FYczNtajd0QUEzVHNCRkFnTUJBQUdqUWpCQU1BNEdBMVVkRHdFQi93UUVBd0lDcERBUEJnTlZIUk1CCkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCVFovZGxCRDdncWpKTU9rR2laSitGL0NXbkh5akFOQmdrcWhraUcKOXcwQkFRc0ZBQU9DQVFFQXU4cVZUZTYzZnd4UkM3cFU2bzRscldSby9ldnB3VkhkV04yT1NES0t4UU5ISURhbQovRDlnamhtdnphQWFKY3E2UmdkaTNCWlRuZDhsSDYzUlZuVHk4RzljUTNTYmFBMHlSQzMyQnQ3bjh6aUYxTTNqCkFxck5zV0hIWkZydkJKWkx2UDBqbms5RmRtSmFYc3o0cGgyRndYQjFTUkdFT3lVcFAvSS9OeVUyNFovRGFvOG4KWlRSazAvbHBVUis2SGp0UGliUTdJejkxcWtxNmsxcEY0SXdlbG5xU1Vqc2FjLzBGa1lib1I0UHkvT1BUbmtIRgpVR2doRUFHRzNnZXFsaGI4Q3FUYmRwKzhLb0R6RzNBOUtRUURBWUlMNkh5Mm9nVEJmU241Uk5WY2pLdHI0QS9UClhnSnlUdjdHZ0FabkZHT1dMc3B2TDhJQVdGYmN4T2hwV2hEaDNRPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
    key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBdmZQYlVPV3RMYU9QYndzTDVSRENWemFXMnliRUVmbzZNd3RrYVVFditnQjhocVBwCjhyU1VPa2EyYzBJbU11WGFIa05Qa1lUaTNPQmsvQmdUN3RKQlh0TS9mSDJjUy85S0l2NENwRlBQdHhEWWRaKzIKUHArYlJENkpiNVh0R0VaM2w4T1k0SkdBblpramRTM3JvMnFNL1NhMnJrOStEaDVkN1VvWU13RVJMbnoxRTlqKwpjbVFMQWJxSUhOQnNEY3RTZklya013cUNqTXlqZDMyN1YxMG9ua3M1NXBUOG5Oc3N4NnBrM1lEK3JndVdwRk00CkRxdlhXbkVTYWxHMEZNbHZSRlR1TlhUL3BmdGNMT29FWVVwL3FFYkFTdkprbEppT3UvRmRlcUxhamtlbmZjbGIKeVZPa1o5ZjlXOTJ6UXlxK3BGeUpkQmtGN041bys3UUFOMDdBUlFJREFRQUJBb0lCQVFDWkFiTHBxUGdrU1JtaQpncTFrS0duQ3dwQWxtMFpZak0wUWpONm5BZ0ZaU2NjRTFVZi9Yb0gvcHpJVUNYYW5qUXB6VWhqbnlMak0zbHU1CnpOTlJqajlsMkpmTStZbEtsaXJyb053VDdnYmxHVWFqQ0xGT0pGWjNWRUIwaDduaDBmRkhhQ0RlMDVWY1hSeDQKcVRLa0FaSHI0S0ZLSzNJSWdXRjdZREc1OCtRWkl0N01BdDQyc1NrUlhaenNyaUVzVWxZNU9xT1dlVHlkYy93dQpDK292V1FlOUU2bnRSc0EvY25nS0M1bnp6T0pCano3SlVTMGQ2UnBGK1Zodk9SRlZ6YTZKMnpRRXJsWWtFdVVUCkJYdThsVmhhcDVOU09nd3ByZXVmb2tNZVY4eGRNdUxuVEthdHVjajIwT3JFRUlHZVFrb08yTzhTeDJtNitOS0kKbzBTT2kzU0pBb0dCQU5LVkk4R0JHQUpSUlV2NVBnVkNDQ1VHSkFBRmtxY0FMTFlaZ0NMZCtLYStBVW41L2VONwpWemR4b2VqNG1UcnFOb0RLTnk5b0N2Z1RBWTlWZ3RZZllpdlNQbW5ZZk9FbTJUR2pLRTJSYzhBd0R5UWdHWFJVCm5WOUVueHRUUHF5dU9tQTFqa0ladXBLeUU0b0drdTE4MDRVaTB2d2pJUWI3cDdML0ZGYksvaGlmQW9HQkFPYnIKclBrOHdicnBlVk9QWmVudzVzbjVhTUpjdDM3emt4dHZzejVSVWlsZnZtVkdja1lyMU41SGpmZjVVVGcwNmNWdgp1N0dGY0NLQmhncFdqSXM3dHZ4cDhUVFM1VEZaWHZqY2VhRC91ZkFiN3ZUVnR0L1hRMFUwQ28rcDdzMENKWlZKCjY2MUtUU1RCYlFtR0xYYmNqNmFYdVM3bzFJM1FRU1l4NW5LV1F5aWJBb0dCQUtBTWZocUtGVWRkb1g5MnRhNmwKV3k5WWxXLzJ6RmxsQnBaNGx5em83QjArK0JmVGl5V2tEc3V5NzgzemMvS1ZKRXVLWlpzQVJxWDVQQXhHZjZSaQpRZWp3YUVObUtMT3ZKUkJXNDBEaE5jcHlQRy9HZmRJdXBWVk5BR2h5UW9aWC9VSTJNaU1IRHdpRGs5b3AyTzNyCkc1QnF3VlNsRm1zS1JaRUQwZCtOZE1ZZEFvR0FkVDNQRXJQd1FHL3RzNmtvdTBBZVRRbWVVS0EyWWZSVkNpY0sKUUdlVmFZQTg4THAxcG43Mmt1eU5mZ3ROVzFZeUlwWDZHOFYrQzJicm9UQVVKMVRvTVB1eEJYclY5dHBEUitMWQp0ZzlnWGpJd2ZvcExVUmJBQnRESFUrMlpXdWp1SC8vcDhvKzQzeUo5czhvMkp4VVFzaXB5VVFqUmNqYjcvT0owCitGU21RR1VDZ1lBdU5XcFhrbVkzbGtRSEFpU3RlblhTT2Zqc0xsbm5XWFJoTDVBYTZqRVZRT3FnUldVQnZ2YWkKK1RQRTNUTHQ5MGwveTZOdjU0dDdrT1QvSlEvREU4WmFiMERlTjdBRzRwRjMwNkxpYlpZNmswc3M1UDNXME8vbAozekJzQ0lEY3BHOWw4bDZSNkUwdHN0Z0I1c25hOE4rRzA2V3Q1R0M0UitRSVd1YTVpUmtVTXc9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
  type: user
sub_kind: user
version: v2`
	databaseClientCAYAML = `
kind: cert_authority
metadata:
  id: 1696989861240620000
  name: me.localhost
spec:
  active_keys:
    tls:
      - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURqVENDQW5XZ0F3SUJBZ0lSQU5XcUtsOWR3WGYrVTBWUmhxNGEyaTB3RFFZSktvWklodmNOQVFFTEJRQXcKWURFVk1CTUdBMVVFQ2hNTWJXVXViRzlqWVd4b2IzTjBNUlV3RXdZRFZRUURFd3h0WlM1c2IyTmhiR2h2YzNReApNREF1QmdOVkJBVVRKekk0TkRBd09URXhNams0TlRBek1qYzRPVEV3TURJNE5qUXlPREV6TmpRMk9UQTVNamt3Ck9UQWVGdzB5TXpFd01URXdNakEwTWpGYUZ3MHpNekV3TURnd01qQTBNakZhTUdBeEZUQVRCZ05WQkFvVERHMWwKTG14dlkyRnNhRzl6ZERFVk1CTUdBMVVFQXhNTWJXVXViRzlqWVd4b2IzTjBNVEF3TGdZRFZRUUZFeWN5T0RRdwpNRGt4TVRJNU9EVXdNekkzT0RreE1EQXlPRFkwTWpneE16WTBOamt3T1RJNU1Ea3dnZ0VpTUEwR0NTcUdTSWIzCkRRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRRGlDNU5GVDhmUy9hSzdSSVVRVnFjVmFxaFBzMFNuU0prTmd3azEKUHZkeFV1OWZZMlNwek5NaUUzSGZlb0Y4S1h2YUU0aHJzMEFGOVRmYlpJTnM1RjNHNTNzOUg3Q2JXWHpOWVRtZApCN0gyWEVxVGp3N0xGL2pzYzkwcTN4ZnZqMkk0Z29tOUdYK3dGMXdaRldjZXVJRkJTdXdCRkV6a1Yzc1o5NEVqClBsWUIxK2lnNlJoWGhvUjdhRlJUNDFvZmtMUUovMDdBVmR4blUranp5VkVFSVk3SjUwUWU3bFc2Nk9wL3BncmwKR1FBSnkwbnowUVpVYVJjVmZrODVHK3NwMnhjcUJ6clJHbXNybmw1TmhMdGJqcUJIUkZ2cU5XS1pLa1V2M0NjUApiTytWT1krV1FmV0UzRThhekxUQ2ppcnJXYWVXeTNLR0RTZGF5YktOK0FKbVpqU3hBZ01CQUFHalFqQkFNQTRHCkExVWREd0VCL3dRRUF3SUJwakFQQmdOVkhSTUJBZjhFQlRBREFRSC9NQjBHQTFVZERnUVdCQlFhVmMzdXlYWnoKdDBWLzFnNzE3MzMrRjFhaHNqQU5CZ2txaGtpRzl3MEJBUXNGQUFPQ0FRRUF6NTVkVnVFdmVLdnJtYThzL0dWSQo2Q0t5akNNYXNjWmhwV1JIT3QxRjQ1T0pjcXg1RDBQeVhSenZXS2NTYzlZTkN0M1BzSi8yNGp3VDlLaElqK2NiClQ5Z0h5WXNkb3pWY2NzMXNZTkFjK3VFSmRSOEsydHJqa1JJN0Q5VmZvTEJJVFlHUkJGTWpSOEE1bENlUzVnTkgKRG42V09rSlpRUi9UQS9IbFFlUmttMW5teUp3VVVQOVA0aUVWVlVSS0lMRVVNTS9EdERXdTZuNnM2K0pVVXNDNwp5QmI2T3JQeVRGbkV4TFljN2RhYUM1bm5UVDZHY2xUSm4wYkJ2UmtXdUFVa1FtWXJyYkpBMnhEVjFBL0JOcmp3Ci9aU2ErU1ZlVWJxSW05ZEVESE4zQUhXcmJzbWwyVjI3YUtrMHVUK0JmeUJBZ3NSdGpMN0U2YUdJanlNcStlOW4KYmc9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
        key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBNGd1VFJVL0gwdjJpdTBTRkVGYW5GV3FvVDdORXAwaVpEWU1KTlQ3M2NWTHZYMk5rCnFjelRJaE54MzNxQmZDbDcyaE9JYTdOQUJmVTMyMlNEYk9SZHh1ZDdQUit3bTFsOHpXRTVuUWV4OWx4S2s0OE8KeXhmNDdIUGRLdDhYNzQ5aU9JS0p2Umwvc0JkY0dSVm5IcmlCUVVyc0FSUk01RmQ3R2ZlQkl6NVdBZGZvb09rWQpWNGFFZTJoVVUrTmFINUMwQ2Y5T3dGWGNaMVBvODhsUkJDR095ZWRFSHU1VnV1anFmNllLNVJrQUNjdEo4OUVHClZHa1hGWDVQT1J2cktkc1hLZ2M2MFJwcks1NWVUWVM3VzQ2Z1IwUmI2alZpbVNwRkw5d25EMnp2bFRtUGxrSDEKaE54UEdzeTB3bzRxNjFtbmxzdHloZzBuV3NteWpmZ0NabVkwc1FJREFRQUJBb0lCQUQ5R01EWkJxOVRDek0rUQowWktPUHZ6K3V4aDhQT1o2cXVVZVhmQjZyTGNiR1FoaGdTY0t2N3NWS0ZYL0s4bStydjJQWkN1SnBJMUdaQmxVCm5IbFp2MnBURjZzM2VLOHpzSHlwRDRDR1MrbURVaGpWL2JVYUE4TGtkKzl0UFgwQWJPVVduVW5Dbm55RFBYT0UKQ3phTlBSa3l5TGRRb0dsMmwyM2dXMVNyT1ZZUVBEUjZncWVJZVFYa3pHYUFUQ2twZWYrOVk4US9pTkZUR05oZAppamtXSUZOdEYzQjdIODUwdnR0VFFRckE3QXQ3ZnN2bmo2YVVDUkQ3MmFYZGJmeHIwK0VQbUR6WGNhejM3U20yClg3ZkJrakRFa0pCa0gwVnBnZHdvMDh3cmtzbnBieUNpbE95alp4Z3lhUWw2NGFKcGVUN1FHbEMxcm1kUEFmTU4KSEdweFBwVUNnWUVBK21vbllBVW1oSVpTR0R6VjI0NFc0QVRnVFdGb2gxb24wKzlZZ2xxR1RUZFFBM2dXYy9ReQowSmJ6QXpOVFZnODNibWhvU3NtbUMwZ1BoSytBb3BCZXc5ZlVxMmhIOVptUzVjZE1CaTJ6cFdvZHQrWXMvNys1Ckk3d3d2bGgvY3llelQvU0ZyazVCVVB0azZFR1pLZk1KdGZlNDcvb05uUzRmc3lmYlAyWDVUWHNDZ1lFQTV4WkcKSmhiYkwwWFljL0plc0JucXBuRFZHNXhkbkh6aGx0aVB3QzdGbHRDRUpuNFdhTUNpSUtsL0o3VGUzbndqMHk0YQpSVzFTWGN6anc5dHZxY0V3aE9CQUZBUHlsd0FWWjUyOTRzdWZzMnk4SHBFcFhhMjlIRXNReW50TE9JRFZyYkVsClJCV1pEb0xhbllVRTNtWml5WnZ0ZU9TT0hTajVhUnIzRTg1UmtNTUNnWUFweUVLUG8reGNXbWtpUUN4U3VPK2EKSzFZZHN5NFV2M2M3eG9qWEh6R2ZlcVl3SGY1cEZJclNBUTNGTC9Bc3dOYzM1ZFhZL0xKbTJYdzFZRzh2TUxXUApLZGtEVEtBTkc3WEYveTN4TGZqMmxiRWx1Uk16RFJOZ0lndGtCeklremEvK25FY2Q0VkxHcDF1YjRTNGtNTGdqCkU1VlkvVGorUys3Z0hydFhaYlZtTndLQmdET3A2eTBBMXlnT2VZSVNvZERGT296VGxSR0ROL3FRZ083MG84N1gKcGgwOXFRM2lDcWlJeUxaOHJvejJCdzIrdTFPdmJ2Z3VwTWVMMHpBcWt5QmtyTEJJWW9zWEJ0bHpqMVdIRXJqdAp4VnFiNk1MOHVUN1VaUDg2V1JxcnpmbG45RjNNeVFRYndBaGFnUDNPaTNRZGQrQ1RGOWg3WUxwc09yYWc3TFJrCjRCOTVBb0dCQU9maHNVSzVSZm1RU1ZzN08wMmMrdWFVelB2dnEwaXNqNW45dWlOaFQxdjFDUGY4YStZdkkyTisKcWV5bHkwRjN3L2sxbElaUzFjWlMwRDVWMUd6bmVHTUgreWYwVFAvNmlUcElHTC94N1pTNGJEZjE3dEc5dklDdgoya1JBTno5WHpzVzFETm9CRkJmZXg5NDFmT0RzdHlvdThvYmF3dDdJWThTdU1GMHV3aDlBCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
  additional_trusted_keys: {}
  cluster_name: me.localhost
  type: db_client
sub_kind: db_client
version: v2`
	databaseCAYAML = `kind: cert_authority
metadata:
  id: 1640648663670001000
  name: me.localhost
spec:
  active_keys:
   tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURpakNDQW5LZ0F3SUJBZ0lRRU9IcEhIZkZwZ28wUndQSVJhdkdpakFOQmdrcWhraUc5dzBCQVFzRkFEQmYKTVJVd0V3WURWUVFLRXd4dFpTNXNiMk5oYkdodmMzUXhGVEFUQmdOVkJBTVRERzFsTG14dlkyRnNhRzl6ZERFdgpNQzBHQTFVRUJSTW1NakkwTkRBMk5ESTNPREkyTWpJNE5UQXdOemMzTmpVeU16STVOak15TWpNeU56VXhORFl3CkhoY05NakV4TWpJM01qTTBOREl6V2hjTk16RXhNakkxTWpNME5ESXpXakJmTVJVd0V3WURWUVFLRXd4dFpTNXMKYjJOaGJHaHZjM1F4RlRBVEJnTlZCQU1UREcxbExteHZZMkZzYUc5emRERXZNQzBHQTFVRUJSTW1NakkwTkRBMgpOREkzT0RJMk1qSTROVEF3TnpjM05qVXlNekk1TmpNeU1qTXlOelV4TkRZd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFETFRrVFkzQ0NMVStNUllkbEMwM2NUTTR6MUpiRGoxYjFQRWdING9iSmwKRjl4NWtQbzhncWNEbmp5L0x5NHdKeUR2Q2xPMkw1T0k3UnYwa1hFUXoybUVEeExnbjJYRG9ZNUh5VFNOVkZHNgpvZ3BlYmhlUFN1aWl0RUNZYUZDZVZFTGNDa1Q0ZGpqRDlwOExNTnJ4MHRPOXdQU1o1OXBLZUxCOG90RFloOHRCCkcyb2EzSGIzTWt0RGxOY0svVE94RFNzRzUrQ2ljdktTa3QrV04xaXJJQ2pvZ2hWTzJGcForRkdxWUM0Y1EwbWMKM0NRaGJwY1o2VTRkWnpGdFJZVzZPYzNucHBOSkZKWXZSSTRIS1FWY0RCM2N4VkhNTUd5Rzc3aFRzdEwvd0RuaQo4U2s5eml4VzN4S2FvUnlrV2FuWno4eC9WdHNydXJqanNzNDV4NlRoem1VWkFnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQnBqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCUmNKK2NRamFQWjZGbEIKcVhoYzYyWXZldGRpQWpBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQVdYNmxZdUhtMmQyMU41RDN1eUJJelFOdApKVFR3b0xnU3FQd09Tbk1EU0luSDkvMjBqZDNGUk9qakh1M3BQRkNLRmE4OVp0ekZxTHZsTjVOdlh2WHFuOXNKCkdudTYzSVo0TWtEZk9sSVZpWFhQWFF4YllHSkMxRVlVU28rTDdtUTY0VnN5UkFpTXdnbmVwMUxwSGhROGYzU2MKeEZoVkNybFJDMmUrNENBai8vOVZWaDdvTEdSMkNhM0xEcFc5VHFxYnB3MEh0QitNcFVqVWxCWnFVbzNVMm5HTQpia1VhSVZKcnNuYk1rYnNsUGQ2dWtVRDlVTHFuUmxJb3A4cjQ1VTdvYVBhR3g3QVFiWndzbGlsNVVJZlppRmlRCm5USk9kYnJHampVdXlRYkM4UUpZY3RhdENjbVBjZUlXMVVWWFVnZ2JsdXl4VjF1NWsyYzlSb1k2RzhiN0FRPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
      key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBeTA1RTJOd2dpMVBqRVdIWlF0TjNFek9NOVNXdzQ5VzlUeElCK0tHeVpSZmNlWkQ2ClBJS25BNTQ4dnk4dU1DY2c3d3BUdGkrVGlPMGI5SkZ4RU05cGhBOFM0SjlsdzZHT1I4azBqVlJSdXFJS1htNFgKajByb29yUkFtR2hRbmxSQzNBcEUrSFk0dy9hZkN6RGE4ZExUdmNEMG1lZmFTbml3ZktMUTJJZkxRUnRxR3R4Mgo5ekpMUTVUWEN2MHpzUTByQnVmZ29uTHlrcExmbGpkWXF5QW82SUlWVHRoYVdmaFJxbUF1SEVOSm5Od2tJVzZYCkdlbE9IV2N4YlVXRnVqbk41NmFUU1JTV0wwU09CeWtGWEF3ZDNNVlJ6REJzaHUrNFU3TFMvOEE1NHZFcFBjNHMKVnQ4U21xRWNwRm1wMmMvTWYxYmJLN3E0NDdMT09jZWs0YzVsR1FJREFRQUJBb0lCQVFDeXNLNXVkTHZkK2ZOQQpHZUtkaThQRENySS8zY3JsMWIwNFBEbWpVR3U5MHdVampEdUU1OGpuc3pMdFR3aW5waHlhUFZkcWI5S2FyTnkvClR2NHpxam14cXBZSys4Nno3ZEZpWXdSZm05YmgxUDZNRlBOOExIamdXTkhWb3dvSXYwS3NxQklLMTgzNDMxRFcKd3pBTkVDS3ZTMk14eXNqZ1g4ZXZKR092alZzbWN1SU1IWFljbVlORlo0dWpuTnc0dnVSWHhlYUtPY2ZWTGZ2ZwoxSWZMK05IYUh6UXI1YVoydkxST1NpdHVId2cvOFpZZ2hQcVFYLy92ak92M0FENzhXVGxINUZXWjExR0hoeCt1Ck9ZUXcwQ1lvTnNiV1UxeklwVTV0cHdCcDRGV0VlVy9jbTY5NXRBVVRlMzR1N3R1MlJJOVBZMEZDWVJ2ZkM2SUQKK2tDNjRFTEpBb0dCQVBSTEt5a0pjdHNGNngzN1liWXdmQm9ZK3V6YU4wV1o1d1RuRmVFaVp1ZzVVYWJUOTdqRApZTnpaYzQ0aitRbkxFUGVDak5tU1FhVHFxK2QvbUVjTnlXS244cVNFcXlOR0R6YlRXQ2tkMGtyWTVwcm9FVVJnClFqbWFwRHFNNEtOSm9jQ1RCS3lueHo2QzZ6ME9uSnhZM3lMdTdyYVBqeE9HTW5rcm9VMDhMVW56QW9HQkFOVU0KU2NsNGh1R0gxK3ZNL0RtenJoQ1NKbUZIcjE2QjhvWExaMHIyNmFKTnJVQ3AwYUFzV2FHd3JLZXZFcDkzRmg5Ygp3QlZYMHE0bXJ3cmZpTGpCYnRzdnlQM3l1ZWs5cWl6M2ZoMWIvZ0k2eWRKVFEzMEplQzB4UzUyRksvR2gwanEvCm43c2Y1bm5DbTlCRW01ZmdSeDFVUmVhOC9vL2M4cTNhbW94c0grdkRBb0dBZUVZUjU5QlpGZkJpQTQ3aVdwcWcKWHhEeGFXOCtTeXdzaTBOaWlFY3h0eCtSVGJ1S2VSTG9PNU5yeXcxMjdSVm5NeFM1Vjkwa0tKZkpMdDZwRUVKLwpaZTBlRDFXcUZHSEgxOHhSMlZ4dlRwNWZXdURxcjJsYzhaTnJTOUJVUU5CZHJMdzFUdlFEcW9rMlhBYzNuOW81CmNhK0ZJNmltWG94eGlTcXI3YVMwLzNVQ2dZQTJBWEJ1N3V1YUhocGcvc3h0UUJ2K3ZWMlhTVm11SmxpNUM4KzYKVkE3emdxZEpmZ0xTakl1SURrWW1GNTRyNkQ4bVlkYTJVbFhvcVl1endPaGlsVDRwdDlwR2JaSXRDdUdwbG05VQp0KzRTMko0eWY4TGEzbHlsY0JxUDZxTXlGR2c3VmpvQ2NGcTNRTnJJbDZ1dGV6L3JzbUlwMUh6Zk1RNGZmZ3V4ClR2Tmtpd0tCZ1FDbVhaSStvTUdQK3U4KzRVaGxjR01NYUNHZi92UVZLdVJYOHlOYVh1bUx6dk1Xajl0cVhpUzcKK1dQUlhuV01RSnd1QldYMzBTcWdFVVdDbjlzOGxWbzh2TVF4MmFtbXhhWkVEMGRoOHNMSkJDNXJoRmVqV29MbQp3cHg5MXR0S3JJODBKMDYyeW90SFpJYkRyQW1LSGZFeE85U1d4T1hUeFVMUGdvTlJVUW5qSnc9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
  additional_trusted_keys: {}
  cluster_name: me.localhost
  signing_alg: 3
  type: db
sub_kind: db
version: v2`
	jwtCAYAML = `kind: cert_authority
metadata:
  id: 1630515580249460000
  name: me.localhost
spec:
  active_keys:
    jwt:
    - private_key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBNWNQaFordnk2N05NbDhwbDJ4SllvSDVFOTl0a1UzVlByTGhnVkhOMmpuZmoyRlhNCm9RdVRKbUJOdWVEd3FiVFpKREJmQ0d1bTU2RVczK1YrQUtsY3pFNVdFS2NBR0wxL3ZvOGU0NDNBRmwzQXRHVVEKSWRHVzRoL0o0Q3BTNG9idVVlMGt3K0JiZnlocUJ2a2lTNmxtUUpjV0hLbzd2aWZhRTJKc1hmZStESklSMS80eApvYjlrbkw1QWVoRS9XV25CY0RiWXROcHlFOHBoQzdGaDgzYmFUdldqbXNWTEZaaWEzKzJqL255aWRzUmUxNGMyCnR4czRWRXp6UVpaYTMwMlpESFRpRmM4MzJtY3lCTEFzRnMycHNsSkxiUnZLdllBS1dLTnRRcnRlSHIwUFBmamEKQjM3cmNWMXlLYTJONjNRTnZrd2FRMWlnbllqSmE3blRQYWpVSndJREFRQUJBb0lCQUY0b1ZMSUt2bVVhK0RObwpMUytHcUMwMU1ieEUreXM4Y3VjOE03WElEM2k0NXZWYnk5emZhbkVhbkIrbGI5cU1FMFJDVWwrWUJqRDhFZXkxCkZscmREUHRveXRwT0picjl4V0RwTStaYXk3SWV2Mzd0djV1c1VXSGZWeEozSmJwUlEwN3RtTmh3ays1Yk9JQWQKRHBIbEhOTXhWMDF0OGNldWV5N2djYnBjY1ZTaXJNYy9ERTlGcFNONFRRWFloMlRDSnVZcTdncFo1SGJ2MlpleApDOHFTTnZqVVlVTithTGlDZ2pBRFhZdGdmV2RUVWN0YkhldVUrL2Ftb3cyUitOVjFWMGpML3lOQm0rdmZUVXNuCmprNFMxQWx1U3FrT0FFaHJhZjM2UzRNcWZQWm8zbXhFOC96TVZTdEY5MTdvQVRKMlZrNzVZaGxINnMzM1BKN1kKMXdDeVdZa0NnWUVBOFRoT1E4M01xTjFTL1VVcUJWeGRaaS9HZFBldmQrbFUvY2FIQVhTNmJyd2RYNEpITTkrbApaczZVWFdocWJHUXFSVlRvZ1dna2VBSHp4R3I1S2QvSTlRRnhEZHE2am1vRUhmU3VNNnNiNU4yVjgyQTVSaUhHCmduS3IzMUpWQS9FdEt3Vk5sZUY3L1VUM0tzWTJzajRZNFkxRnI3amRkOS9tdTVaNWVwODFidFVDZ1lFQTg5Zm0KSE9tc0JYZUQ0M0FHVmltSFhEWWVGckI3UWRhY3hOQ3R4NjZuTk5jMFpxM0pDZGtDMjVaTG1nQytON2JEbkRjMApNZ2NPUGdyUWxVdWpRbWl1d1daeFlXcDAvWDh3STFHZ2p4RkEvRkJvNjFnd3lwR2tqbjJlS3lVM1ovWlJBRTBjCmlxaW5UTVc5TFBlV1k0MEhOUzFic3E2YmFoeXg1bTFVV0gxclRRc0NnWUF3bE1WMmRHbEdqU1NjcTZSVjVnOU4KZUV2QTNPMXkrZ1JMQkFQR3NFcW42SzBGd2tneTAxVU5pb2RvOUpHU2VPM21mcjVBNmNlR2YrWW5aZC8rcGZwawpGY0UrS0JJd2dudUh5UEtZcDFwNzBvRFR2a3Bxckh5OVl2am9oajFuQ05pdTlHZDJ5eTNjaVZvNlBDZGg2STI4ClIyYUVpSGZhSDdicGl0bTJiNEFrYlFLQmdRQ2tmUUpraEppZkEzVTdpa2tyL0Uyc1BYRmttdDQ2bG53Z0pDam0KSjRIeG1pNW1DVnN4UW11MEZ4bWVwRnVzbDZReWorYXN6S2VsNElPK0FrejZNa1dZZnZPQzVGNVExbWh4bXRHMQpVTTFHcHpOdmRvbExUSjMxNVBVNlk1dVJqTTR0WnRjWERoZjFLUHFwQjhjeUZtTkRVdnFsZVRXcmlmblQxL0pxCjB3Zjc2d0tCZ1FDN3hIK3NXbTR6cjVRb1VvMWlrYlRMSHRBaVFETWE2TlpWTTk0MHcxZHExUWk4YXpMTE1lMUcKUVduU1ZqTDVLaVlSY0toSjVYOGpoQjF5a2U1OTR1ZU1CSFJRdjFRU3RCalRkSzIxN2tnTTFTRmZEbzFFQWpRdgpMWk5pWHROYXVDeG9BMi96SHJrZ0d0NGQzQldKZk5ENkJPQ1RKK2lSRnNWc1VvcjhITzJQR2c9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
      public_key: LS0tLS1CRUdJTiBSU0EgUFVCTElDIEtFWS0tLS0tCk1JSUJDZ0tDQVFFQTVjUGhaK3Z5NjdOTWw4cGwyeEpZb0g1RTk5dGtVM1ZQckxoZ1ZITjJqbmZqMkZYTW9RdVQKSm1CTnVlRHdxYlRaSkRCZkNHdW01NkVXMytWK0FLbGN6RTVXRUtjQUdMMS92bzhlNDQzQUZsM0F0R1VRSWRHVwo0aC9KNENwUzRvYnVVZTBrdytCYmZ5aHFCdmtpUzZsbVFKY1dIS283dmlmYUUySnNYZmUrREpJUjEvNHhvYjlrCm5MNUFlaEUvV1duQmNEYll0TnB5RThwaEM3Rmg4M2JhVHZXam1zVkxGWmlhMysyai9ueWlkc1JlMTRjMnR4czQKVkV6elFaWmEzMDJaREhUaUZjODMybWN5QkxBc0ZzMnBzbEpMYlJ2S3ZZQUtXS050UXJ0ZUhyMFBQZmphQjM3cgpjVjF5S2EyTjYzUU52a3dhUTFpZ25ZakphN25UUGFqVUp3SURBUUFCCi0tLS0tRU5EIFJTQSBQVUJMSUMgS0VZLS0tLS0K
  additional_trusted_keys: {}
  cluster_name: me.localhost
  jwt_key_pairs:
  - private_key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBNWNQaFordnk2N05NbDhwbDJ4SllvSDVFOTl0a1UzVlByTGhnVkhOMmpuZmoyRlhNCm9RdVRKbUJOdWVEd3FiVFpKREJmQ0d1bTU2RVczK1YrQUtsY3pFNVdFS2NBR0wxL3ZvOGU0NDNBRmwzQXRHVVEKSWRHVzRoL0o0Q3BTNG9idVVlMGt3K0JiZnlocUJ2a2lTNmxtUUpjV0hLbzd2aWZhRTJKc1hmZStESklSMS80eApvYjlrbkw1QWVoRS9XV25CY0RiWXROcHlFOHBoQzdGaDgzYmFUdldqbXNWTEZaaWEzKzJqL255aWRzUmUxNGMyCnR4czRWRXp6UVpaYTMwMlpESFRpRmM4MzJtY3lCTEFzRnMycHNsSkxiUnZLdllBS1dLTnRRcnRlSHIwUFBmamEKQjM3cmNWMXlLYTJONjNRTnZrd2FRMWlnbllqSmE3blRQYWpVSndJREFRQUJBb0lCQUY0b1ZMSUt2bVVhK0RObwpMUytHcUMwMU1ieEUreXM4Y3VjOE03WElEM2k0NXZWYnk5emZhbkVhbkIrbGI5cU1FMFJDVWwrWUJqRDhFZXkxCkZscmREUHRveXRwT0picjl4V0RwTStaYXk3SWV2Mzd0djV1c1VXSGZWeEozSmJwUlEwN3RtTmh3ays1Yk9JQWQKRHBIbEhOTXhWMDF0OGNldWV5N2djYnBjY1ZTaXJNYy9ERTlGcFNONFRRWFloMlRDSnVZcTdncFo1SGJ2MlpleApDOHFTTnZqVVlVTithTGlDZ2pBRFhZdGdmV2RUVWN0YkhldVUrL2Ftb3cyUitOVjFWMGpML3lOQm0rdmZUVXNuCmprNFMxQWx1U3FrT0FFaHJhZjM2UzRNcWZQWm8zbXhFOC96TVZTdEY5MTdvQVRKMlZrNzVZaGxINnMzM1BKN1kKMXdDeVdZa0NnWUVBOFRoT1E4M01xTjFTL1VVcUJWeGRaaS9HZFBldmQrbFUvY2FIQVhTNmJyd2RYNEpITTkrbApaczZVWFdocWJHUXFSVlRvZ1dna2VBSHp4R3I1S2QvSTlRRnhEZHE2am1vRUhmU3VNNnNiNU4yVjgyQTVSaUhHCmduS3IzMUpWQS9FdEt3Vk5sZUY3L1VUM0tzWTJzajRZNFkxRnI3amRkOS9tdTVaNWVwODFidFVDZ1lFQTg5Zm0KSE9tc0JYZUQ0M0FHVmltSFhEWWVGckI3UWRhY3hOQ3R4NjZuTk5jMFpxM0pDZGtDMjVaTG1nQytON2JEbkRjMApNZ2NPUGdyUWxVdWpRbWl1d1daeFlXcDAvWDh3STFHZ2p4RkEvRkJvNjFnd3lwR2tqbjJlS3lVM1ovWlJBRTBjCmlxaW5UTVc5TFBlV1k0MEhOUzFic3E2YmFoeXg1bTFVV0gxclRRc0NnWUF3bE1WMmRHbEdqU1NjcTZSVjVnOU4KZUV2QTNPMXkrZ1JMQkFQR3NFcW42SzBGd2tneTAxVU5pb2RvOUpHU2VPM21mcjVBNmNlR2YrWW5aZC8rcGZwawpGY0UrS0JJd2dudUh5UEtZcDFwNzBvRFR2a3Bxckh5OVl2am9oajFuQ05pdTlHZDJ5eTNjaVZvNlBDZGg2STI4ClIyYUVpSGZhSDdicGl0bTJiNEFrYlFLQmdRQ2tmUUpraEppZkEzVTdpa2tyL0Uyc1BYRmttdDQ2bG53Z0pDam0KSjRIeG1pNW1DVnN4UW11MEZ4bWVwRnVzbDZReWorYXN6S2VsNElPK0FrejZNa1dZZnZPQzVGNVExbWh4bXRHMQpVTTFHcHpOdmRvbExUSjMxNVBVNlk1dVJqTTR0WnRjWERoZjFLUHFwQjhjeUZtTkRVdnFsZVRXcmlmblQxL0pxCjB3Zjc2d0tCZ1FDN3hIK3NXbTR6cjVRb1VvMWlrYlRMSHRBaVFETWE2TlpWTTk0MHcxZHExUWk4YXpMTE1lMUcKUVduU1ZqTDVLaVlSY0toSjVYOGpoQjF5a2U1OTR1ZU1CSFJRdjFRU3RCalRkSzIxN2tnTTFTRmZEbzFFQWpRdgpMWk5pWHROYXVDeG9BMi96SHJrZ0d0NGQzQldKZk5ENkJPQ1RKK2lSRnNWc1VvcjhITzJQR2c9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
    public_key: LS0tLS1CRUdJTiBSU0EgUFVCTElDIEtFWS0tLS0tCk1JSUJDZ0tDQVFFQTVjUGhaK3Z5NjdOTWw4cGwyeEpZb0g1RTk5dGtVM1ZQckxoZ1ZITjJqbmZqMkZYTW9RdVQKSm1CTnVlRHdxYlRaSkRCZkNHdW01NkVXMytWK0FLbGN6RTVXRUtjQUdMMS92bzhlNDQzQUZsM0F0R1VRSWRHVwo0aC9KNENwUzRvYnVVZTBrdytCYmZ5aHFCdmtpUzZsbVFKY1dIS283dmlmYUUySnNYZmUrREpJUjEvNHhvYjlrCm5MNUFlaEUvV1duQmNEYll0TnB5RThwaEM3Rmg4M2JhVHZXam1zVkxGWmlhMysyai9ueWlkc1JlMTRjMnR4czQKVkV6elFaWmEzMDJaREhUaUZjODMybWN5QkxBc0ZzMnBzbEpMYlJ2S3ZZQUtXS050UXJ0ZUhyMFBQZmphQjM3cgpjVjF5S2EyTjYzUU52a3dhUTFpZ25ZakphN25UUGFqVUp3SURBUUFCCi0tLS0tRU5EIFJTQSBQVUJMSUMgS0VZLS0tLS0K
  type: jwt
sub_kind: jwt
version: v2`
	openSSHCAYAML = `kind: cert_authority
metadata:
  id: 1630513579536527000
  name: me.localhost
spec:
  active_keys:
    ssh:
    - private_key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBeHpRdWtjUlhCR09nRDUrS3B1MGRrdVpXVjVsc294Wlp1cFRhRHdFcmYvQ1prOWFqCjRaZzJoeTBoUHl5SXRBRytIN08xRVYwZ0taWVRDWWs4REtiaFQ4ZWl3QldDNUhKaGtpYW9ZRStSUXpQTHd3STcKcElmb0t4ejR2WG5MRnNvWHVVY2hLTmhYdk1RZU9pVVJ3OVQ3L0RSREE0dDdsazlPY01jWE9ySHV3L2JxTTA0VApmTmhVeXhHVmhyTC90aFpQNDE4b04xZjZKQzZMU3grZThIM01iSEQralVGMjBVbU8wVUFLUHUwRXVQS3hkS2loCm1tbVZDZ3FaRWhBWjhrcklVYlV5R0tUemZjQyt4VFNrUEN3UE1qejg1aTJQVVBYS3hmVE00cW54QVh5VzRCSzYKeXo1Znk5UTgyalVESi9VS0JvSU92N1hUU2JhNUpDUHdiUUx4cHdJREFRQUJBb0lCQUJydU53MkYyYTNDT2pWaQpnRUFvOWtLUjJVSm1mNFZjMUN5aFN3bVVRdWs5QWNZMjBsa0JWdjNYWUJOR1ZnVGY1M0FwdjJUbGpoK1JKbW0zCm4rS2wvUGZvS1Z5R2kvZU9ieHB2RjN4TnhYbXNXdk8yTFpJRXZhSjJmRHBCYU85Znl1MUZiSG8xSlVkanpDSlkKT0pxZEJLUUgvTGRSK0JkT0NYQzl1YW81dStuS0RxZTZvaE1FZm5VUUpmeGd4QzNwMHhzRGkvNGovYWhCV3cvaApoZk02TUloaUlmZEhWNlN1MWlPQWlNbXNtcDBWSkd4OVR5SG9UZFdKWHkxYjRTNDh5L21rNjdNeThzUDJ0SXRHCksvdGxaaDFteDYwbCtscnZkTDZFVW9CVDVScXdCZDBCN0g3c1V0UkZjVVVkMVZBNXpMYVowYnhkbHVpL1dEMlcKaHVtOHJpRUNnWUVBN0hPZlN3OVRKQzIxSHFwb2kvUHRKOGZ5QjVMRXJaQTBmY21vd2VtMG55dTRyMVkxN2FSTwp6bU5qYmZtYWRrOU95RnJNaS9Ydm5pbW1wcnNmK3Z1dEVkeEJEK3JYMGJPZk52ZEl5NHNMNEFaN2JZSkdUVEk0ClVFREdPRVF2UkpUQmxhQ3hvMGVEcGVSbTZqamhjYjdmcHV6K3ZCaVB2ZFBHbExxU0JuaTczSDBDZ1lFQTE2dzYKazhzUDlCK251MFFkSVdRano0MU9wTzJlb2FKenhndFVRTnkyV0VuRi9ydk51K21ra1BtL0lWbWZvdzFOYXAwNApGL29FeHpUSWw2WkFCcUdjWUtHVDRNSjRlVS9QT2RsbWZtSE5DcThHTGs4TlRBaC9UdVFuZ0VKTDB2UE9Ta0wzCjMrWHg2N010MDhVUXNmRjlDQU8yS3BuQzhOOEw5NmhjOXp6ZTgvTUNnWUEwakptNVQ4V1ZnOGI5OHJkYmF6R28KcHFvbWZyclJLL3hPZkZQU0RNT0VvRzNpSWRISVo3elA1NHpBY3ptZDA1Qlp2THc2MnNTUExRaUpnNHJlOTdJRwpCeUk2akdHOGpDUDFUazNTVnF1ajlTelhNSjI1S0ZFVm5OK3d2NDZWdWsydm1GQUNUckYyVytWM1puN01EYlNjCjM0elpkc2Z6VXk2Ti9Vell2VnBhN1FLQmdRQ3NOZC9JSnpxajZhcmJBdlpudFRoTEFFQXR2WGNQQlZLQWJvZG0KQzFhbWhMSE9SMU50bXBCSEdzU2M4cDFmYXIzSVJhV0dyNktsRmVhZUFLZmJJNnhrRkdDcDlWNlJMMEwrcERNTQo4emJ3TXZVeWdQalRIMjNZSnFITDdpUHhXNi82NkNKWTY1a1NaVTVRYkdoNlRhTlNoUFF1YS95V3JPTTNhMzVnCkJJRGFOUUtCZ0dBSE5EOWVjcGpySElxUFgzekZUYm1uZjZORWl1SmN0aHpPN0hLZzI0Slh3V0FoRWJBaXpUYjQKRnY1VmY0d2dKc2xGSjFmNDdhVjJxSU5ncWJ5ajBMSlNjYksyOGlXVmIwZmxSbnJEZnd0amxaS2lneWRDTlZubApteUNBS2IyOTlZL1d0Y3NobjUxa29DdWtzSVA1VzJwZjk5M2pLbEw0OWNPN0l6N3VKbTEzCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
      public_key: c3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCQVFESE5DNlJ4RmNFWTZBUG40cW03UjJTNWxaWG1XeWpGbG02bE5vUEFTdC84Sm1UMXFQaG1EYUhMU0UvTElpMEFiNGZzN1VSWFNBcGxoTUppVHdNcHVGUHg2TEFGWUxrY21HU0pxaGdUNUZETTh2REFqdWtoK2dySFBpOWVjc1d5aGU1UnlFbzJGZTh4QjQ2SlJIRDFQdjhORU1EaTN1V1QwNXd4eGM2c2U3RDl1b3pUaE44MkZUTEVaV0dzdisyRmsvalh5ZzNWL29rTG90TEg1N3dmY3hzY1A2TlFYYlJTWTdSUUFvKzdRUzQ4ckYwcUtHYWFaVUtDcGtTRUJueVNzaFJ0VElZcFBOOXdMN0ZOS1E4TEE4eVBQem1MWTlROWNyRjlNemlxZkVCZkpiZ0VyckxQbC9MMUR6YU5RTW45UW9HZ2c2L3RkTkp0cmtrSS9CdEF2R24K
  additional_trusted_keys: {}
  checking_keys:
  - c3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCQVFESE5DNlJ4RmNFWTZBUG40cW03UjJTNWxaWG1XeWpGbG02bE5vUEFTdC84Sm1UMXFQaG1EYUhMU0UvTElpMEFiNGZzN1VSWFNBcGxoTUppVHdNcHVGUHg2TEFGWUxrY21HU0pxaGdUNUZETTh2REFqdWtoK2dySFBpOWVjc1d5aGU1UnlFbzJGZTh4QjQ2SlJIRDFQdjhORU1EaTN1V1QwNXd4eGM2c2U3RDl1b3pUaE44MkZUTEVaV0dzdisyRmsvalh5ZzNWL29rTG90TEg1N3dmY3hzY1A2TlFYYlJTWTdSUUFvKzdRUzQ4ckYwcUtHYWFaVUtDcGtTRUJueVNzaFJ0VElZcFBOOXdMN0ZOS1E4TEE4eVBQem1MWTlROWNyRjlNemlxZkVCZkpiZ0VyckxQbC9MMUR6YU5RTW45UW9HZ2c2L3RkTkp0cmtrSS9CdEF2R24K
  cluster_name: me.localhost
  signing_alg: 3
  signing_keys:
  - LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBeHpRdWtjUlhCR09nRDUrS3B1MGRrdVpXVjVsc294Wlp1cFRhRHdFcmYvQ1prOWFqCjRaZzJoeTBoUHl5SXRBRytIN08xRVYwZ0taWVRDWWs4REtiaFQ4ZWl3QldDNUhKaGtpYW9ZRStSUXpQTHd3STcKcElmb0t4ejR2WG5MRnNvWHVVY2hLTmhYdk1RZU9pVVJ3OVQ3L0RSREE0dDdsazlPY01jWE9ySHV3L2JxTTA0VApmTmhVeXhHVmhyTC90aFpQNDE4b04xZjZKQzZMU3grZThIM01iSEQralVGMjBVbU8wVUFLUHUwRXVQS3hkS2loCm1tbVZDZ3FaRWhBWjhrcklVYlV5R0tUemZjQyt4VFNrUEN3UE1qejg1aTJQVVBYS3hmVE00cW54QVh5VzRCSzYKeXo1Znk5UTgyalVESi9VS0JvSU92N1hUU2JhNUpDUHdiUUx4cHdJREFRQUJBb0lCQUJydU53MkYyYTNDT2pWaQpnRUFvOWtLUjJVSm1mNFZjMUN5aFN3bVVRdWs5QWNZMjBsa0JWdjNYWUJOR1ZnVGY1M0FwdjJUbGpoK1JKbW0zCm4rS2wvUGZvS1Z5R2kvZU9ieHB2RjN4TnhYbXNXdk8yTFpJRXZhSjJmRHBCYU85Znl1MUZiSG8xSlVkanpDSlkKT0pxZEJLUUgvTGRSK0JkT0NYQzl1YW81dStuS0RxZTZvaE1FZm5VUUpmeGd4QzNwMHhzRGkvNGovYWhCV3cvaApoZk02TUloaUlmZEhWNlN1MWlPQWlNbXNtcDBWSkd4OVR5SG9UZFdKWHkxYjRTNDh5L21rNjdNeThzUDJ0SXRHCksvdGxaaDFteDYwbCtscnZkTDZFVW9CVDVScXdCZDBCN0g3c1V0UkZjVVVkMVZBNXpMYVowYnhkbHVpL1dEMlcKaHVtOHJpRUNnWUVBN0hPZlN3OVRKQzIxSHFwb2kvUHRKOGZ5QjVMRXJaQTBmY21vd2VtMG55dTRyMVkxN2FSTwp6bU5qYmZtYWRrOU95RnJNaS9Ydm5pbW1wcnNmK3Z1dEVkeEJEK3JYMGJPZk52ZEl5NHNMNEFaN2JZSkdUVEk0ClVFREdPRVF2UkpUQmxhQ3hvMGVEcGVSbTZqamhjYjdmcHV6K3ZCaVB2ZFBHbExxU0JuaTczSDBDZ1lFQTE2dzYKazhzUDlCK251MFFkSVdRano0MU9wTzJlb2FKenhndFVRTnkyV0VuRi9ydk51K21ra1BtL0lWbWZvdzFOYXAwNApGL29FeHpUSWw2WkFCcUdjWUtHVDRNSjRlVS9QT2RsbWZtSE5DcThHTGs4TlRBaC9UdVFuZ0VKTDB2UE9Ta0wzCjMrWHg2N010MDhVUXNmRjlDQU8yS3BuQzhOOEw5NmhjOXp6ZTgvTUNnWUEwakptNVQ4V1ZnOGI5OHJkYmF6R28KcHFvbWZyclJLL3hPZkZQU0RNT0VvRzNpSWRISVo3elA1NHpBY3ptZDA1Qlp2THc2MnNTUExRaUpnNHJlOTdJRwpCeUk2akdHOGpDUDFUazNTVnF1ajlTelhNSjI1S0ZFVm5OK3d2NDZWdWsydm1GQUNUckYyVytWM1puN01EYlNjCjM0elpkc2Z6VXk2Ti9Vell2VnBhN1FLQmdRQ3NOZC9JSnpxajZhcmJBdlpudFRoTEFFQXR2WGNQQlZLQWJvZG0KQzFhbWhMSE9SMU50bXBCSEdzU2M4cDFmYXIzSVJhV0dyNktsRmVhZUFLZmJJNnhrRkdDcDlWNlJMMEwrcERNTQo4emJ3TXZVeWdQalRIMjNZSnFITDdpUHhXNi82NkNKWTY1a1NaVTVRYkdoNlRhTlNoUFF1YS95V3JPTTNhMzVnCkJJRGFOUUtCZ0dBSE5EOWVjcGpySElxUFgzekZUYm1uZjZORWl1SmN0aHpPN0hLZzI0Slh3V0FoRWJBaXpUYjQKRnY1VmY0d2dKc2xGSjFmNDdhVjJxSU5ncWJ5ajBMSlNjYksyOGlXVmIwZmxSbnJEZnd0amxaS2lneWRDTlZubApteUNBS2IyOTlZL1d0Y3NobjUxa29DdWtzSVA1VzJwZjk5M2pLbEw0OWNPN0l6N3VKbTEzCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
  type: openssh
sub_kind: openssh
version: v2`
	samlCAYAML = `kind: cert_authority
metadata:
  id: 1640648663670002000
  name: me.localhost
spec:
  active_keys:
   tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURpakNDQW5LZ0F3SUJBZ0lRRU9IcEhIZkZwZ28wUndQSVJhdkdpakFOQmdrcWhraUc5dzBCQVFzRkFEQmYKTVJVd0V3WURWUVFLRXd4dFpTNXNiMk5oYkdodmMzUXhGVEFUQmdOVkJBTVRERzFsTG14dlkyRnNhRzl6ZERFdgpNQzBHQTFVRUJSTW1NakkwTkRBMk5ESTNPREkyTWpJNE5UQXdOemMzTmpVeU16STVOak15TWpNeU56VXhORFl3CkhoY05NakV4TWpJM01qTTBOREl6V2hjTk16RXhNakkxTWpNME5ESXpXakJmTVJVd0V3WURWUVFLRXd4dFpTNXMKYjJOaGJHaHZjM1F4RlRBVEJnTlZCQU1UREcxbExteHZZMkZzYUc5emRERXZNQzBHQTFVRUJSTW1NakkwTkRBMgpOREkzT0RJMk1qSTROVEF3TnpjM05qVXlNekk1TmpNeU1qTXlOelV4TkRZd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFETFRrVFkzQ0NMVStNUllkbEMwM2NUTTR6MUpiRGoxYjFQRWdING9iSmwKRjl4NWtQbzhncWNEbmp5L0x5NHdKeUR2Q2xPMkw1T0k3UnYwa1hFUXoybUVEeExnbjJYRG9ZNUh5VFNOVkZHNgpvZ3BlYmhlUFN1aWl0RUNZYUZDZVZFTGNDa1Q0ZGpqRDlwOExNTnJ4MHRPOXdQU1o1OXBLZUxCOG90RFloOHRCCkcyb2EzSGIzTWt0RGxOY0svVE94RFNzRzUrQ2ljdktTa3QrV04xaXJJQ2pvZ2hWTzJGcForRkdxWUM0Y1EwbWMKM0NRaGJwY1o2VTRkWnpGdFJZVzZPYzNucHBOSkZKWXZSSTRIS1FWY0RCM2N4VkhNTUd5Rzc3aFRzdEwvd0RuaQo4U2s5eml4VzN4S2FvUnlrV2FuWno4eC9WdHNydXJqanNzNDV4NlRoem1VWkFnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQnBqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCUmNKK2NRamFQWjZGbEIKcVhoYzYyWXZldGRpQWpBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQVdYNmxZdUhtMmQyMU41RDN1eUJJelFOdApKVFR3b0xnU3FQd09Tbk1EU0luSDkvMjBqZDNGUk9qakh1M3BQRkNLRmE4OVp0ekZxTHZsTjVOdlh2WHFuOXNKCkdudTYzSVo0TWtEZk9sSVZpWFhQWFF4YllHSkMxRVlVU28rTDdtUTY0VnN5UkFpTXdnbmVwMUxwSGhROGYzU2MKeEZoVkNybFJDMmUrNENBai8vOVZWaDdvTEdSMkNhM0xEcFc5VHFxYnB3MEh0QitNcFVqVWxCWnFVbzNVMm5HTQpia1VhSVZKcnNuYk1rYnNsUGQ2dWtVRDlVTHFuUmxJb3A4cjQ1VTdvYVBhR3g3QVFiWndzbGlsNVVJZlppRmlRCm5USk9kYnJHampVdXlRYkM4UUpZY3RhdENjbVBjZUlXMVVWWFVnZ2JsdXl4VjF1NWsyYzlSb1k2RzhiN0FRPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
      key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBeTA1RTJOd2dpMVBqRVdIWlF0TjNFek9NOVNXdzQ5VzlUeElCK0tHeVpSZmNlWkQ2ClBJS25BNTQ4dnk4dU1DY2c3d3BUdGkrVGlPMGI5SkZ4RU05cGhBOFM0SjlsdzZHT1I4azBqVlJSdXFJS1htNFgKajByb29yUkFtR2hRbmxSQzNBcEUrSFk0dy9hZkN6RGE4ZExUdmNEMG1lZmFTbml3ZktMUTJJZkxRUnRxR3R4Mgo5ekpMUTVUWEN2MHpzUTByQnVmZ29uTHlrcExmbGpkWXF5QW82SUlWVHRoYVdmaFJxbUF1SEVOSm5Od2tJVzZYCkdlbE9IV2N4YlVXRnVqbk41NmFUU1JTV0wwU09CeWtGWEF3ZDNNVlJ6REJzaHUrNFU3TFMvOEE1NHZFcFBjNHMKVnQ4U21xRWNwRm1wMmMvTWYxYmJLN3E0NDdMT09jZWs0YzVsR1FJREFRQUJBb0lCQVFDeXNLNXVkTHZkK2ZOQQpHZUtkaThQRENySS8zY3JsMWIwNFBEbWpVR3U5MHdVampEdUU1OGpuc3pMdFR3aW5waHlhUFZkcWI5S2FyTnkvClR2NHpxam14cXBZSys4Nno3ZEZpWXdSZm05YmgxUDZNRlBOOExIamdXTkhWb3dvSXYwS3NxQklLMTgzNDMxRFcKd3pBTkVDS3ZTMk14eXNqZ1g4ZXZKR092alZzbWN1SU1IWFljbVlORlo0dWpuTnc0dnVSWHhlYUtPY2ZWTGZ2ZwoxSWZMK05IYUh6UXI1YVoydkxST1NpdHVId2cvOFpZZ2hQcVFYLy92ak92M0FENzhXVGxINUZXWjExR0hoeCt1Ck9ZUXcwQ1lvTnNiV1UxeklwVTV0cHdCcDRGV0VlVy9jbTY5NXRBVVRlMzR1N3R1MlJJOVBZMEZDWVJ2ZkM2SUQKK2tDNjRFTEpBb0dCQVBSTEt5a0pjdHNGNngzN1liWXdmQm9ZK3V6YU4wV1o1d1RuRmVFaVp1ZzVVYWJUOTdqRApZTnpaYzQ0aitRbkxFUGVDak5tU1FhVHFxK2QvbUVjTnlXS244cVNFcXlOR0R6YlRXQ2tkMGtyWTVwcm9FVVJnClFqbWFwRHFNNEtOSm9jQ1RCS3lueHo2QzZ6ME9uSnhZM3lMdTdyYVBqeE9HTW5rcm9VMDhMVW56QW9HQkFOVU0KU2NsNGh1R0gxK3ZNL0RtenJoQ1NKbUZIcjE2QjhvWExaMHIyNmFKTnJVQ3AwYUFzV2FHd3JLZXZFcDkzRmg5Ygp3QlZYMHE0bXJ3cmZpTGpCYnRzdnlQM3l1ZWs5cWl6M2ZoMWIvZ0k2eWRKVFEzMEplQzB4UzUyRksvR2gwanEvCm43c2Y1bm5DbTlCRW01ZmdSeDFVUmVhOC9vL2M4cTNhbW94c0grdkRBb0dBZUVZUjU5QlpGZkJpQTQ3aVdwcWcKWHhEeGFXOCtTeXdzaTBOaWlFY3h0eCtSVGJ1S2VSTG9PNU5yeXcxMjdSVm5NeFM1Vjkwa0tKZkpMdDZwRUVKLwpaZTBlRDFXcUZHSEgxOHhSMlZ4dlRwNWZXdURxcjJsYzhaTnJTOUJVUU5CZHJMdzFUdlFEcW9rMlhBYzNuOW81CmNhK0ZJNmltWG94eGlTcXI3YVMwLzNVQ2dZQTJBWEJ1N3V1YUhocGcvc3h0UUJ2K3ZWMlhTVm11SmxpNUM4KzYKVkE3emdxZEpmZ0xTakl1SURrWW1GNTRyNkQ4bVlkYTJVbFhvcVl1endPaGlsVDRwdDlwR2JaSXRDdUdwbG05VQp0KzRTMko0eWY4TGEzbHlsY0JxUDZxTXlGR2c3VmpvQ2NGcTNRTnJJbDZ1dGV6L3JzbUlwMUh6Zk1RNGZmZ3V4ClR2Tmtpd0tCZ1FDbVhaSStvTUdQK3U4KzRVaGxjR01NYUNHZi92UVZLdVJYOHlOYVh1bUx6dk1Xajl0cVhpUzcKK1dQUlhuV01RSnd1QldYMzBTcWdFVVdDbjlzOGxWbzh2TVF4MmFtbXhhWkVEMGRoOHNMSkJDNXJoRmVqV29MbQp3cHg5MXR0S3JJODBKMDYyeW90SFpJYkRyQW1LSGZFeE85U1d4T1hUeFVMUGdvTlJVUW5qSnc9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
  additional_trusted_keys: {}
  cluster_name: me.localhost
  signing_alg: 3
  type: saml_idp
sub_kind: saml_idp
version: v2`
)

func TestInit_bootstrap(t *testing.T) {
	t.Parallel()

	hostCA := resourceFromYAML(t, hostCAYAML).(types.CertAuthority)
	userCA := resourceFromYAML(t, userCAYAML).(types.CertAuthority)
	jwtCA := resourceFromYAML(t, jwtCAYAML).(types.CertAuthority)
	dbServerCA := resourceFromYAML(t, databaseCAYAML).(types.CertAuthority)
	dbClientCA := resourceFromYAML(t, databaseClientCAYAML).(types.CertAuthority)
	osshCA := resourceFromYAML(t, openSSHCAYAML).(types.CertAuthority)
	samlCA := resourceFromYAML(t, samlCAYAML).(types.CertAuthority)

	invalidHostCA := resourceFromYAML(t, hostCAYAML).(types.CertAuthority)
	invalidHostCA.(*types.CertAuthorityV2).Spec.ActiveKeys.SSH = nil
	invalidUserCA := resourceFromYAML(t, userCAYAML).(types.CertAuthority)
	invalidUserCA.(*types.CertAuthorityV2).Spec.ActiveKeys.SSH = nil
	invalidJWTCA := resourceFromYAML(t, jwtCAYAML).(types.CertAuthority)
	invalidJWTCA.(*types.CertAuthorityV2).Spec.ActiveKeys.JWT = nil
	invalidDBServerCA := resourceFromYAML(t, databaseCAYAML).(types.CertAuthority)
	invalidDBServerCA.(*types.CertAuthorityV2).Spec.ActiveKeys.TLS = nil
	invalidDBClientCA := resourceFromYAML(t, databaseClientCAYAML).(types.CertAuthority)
	invalidDBClientCA.(*types.CertAuthorityV2).Spec.ActiveKeys.TLS = nil
	invalidOSSHCA := resourceFromYAML(t, openSSHCAYAML).(types.CertAuthority)
	invalidOSSHCA.(*types.CertAuthorityV2).Spec.ActiveKeys.SSH = nil
	invalidSAMLCA := resourceFromYAML(t, samlCAYAML).(types.CertAuthority)
	invalidSAMLCA.(*types.CertAuthorityV2).Spec.ActiveKeys.TLS = nil

	tests := []struct {
		name         string
		modifyConfig func(*InitConfig)
		assertError  require.ErrorAssertionFunc
	}{
		{
			// Issue https://github.com/gravitational/teleport/issues/7853.
			name: "OK bootstrap CAs",
			modifyConfig: func(cfg *InitConfig) {
				cfg.BootstrapResources = append(
					cfg.BootstrapResources,
					hostCA.Clone(),
					userCA.Clone(),
					jwtCA.Clone(),
					dbServerCA.Clone(),
					dbClientCA.Clone(),
					osshCA.Clone(),
					samlCA.Clone(),
				)
			},
			assertError: require.NoError,
		},
		{
			name: "NOK bootstrap Host CA missing keys",
			modifyConfig: func(cfg *InitConfig) {
				cfg.BootstrapResources = append(
					cfg.BootstrapResources,
					invalidHostCA.Clone(),
					userCA.Clone(),
					jwtCA.Clone(),
					dbServerCA.Clone(),
					dbClientCA.Clone(),
					osshCA.Clone(),
					samlCA.Clone(),
				)
			},
			assertError: require.Error,
		},
		{
			name: "NOK bootstrap User CA missing keys",
			modifyConfig: func(cfg *InitConfig) {
				cfg.BootstrapResources = append(
					cfg.BootstrapResources,
					hostCA.Clone(),
					invalidUserCA.Clone(),
					jwtCA.Clone(),
					dbServerCA.Clone(),
					dbClientCA.Clone(),
					osshCA.Clone(),
					samlCA.Clone(),
				)
			},
			assertError: require.Error,
		},
		{
			name: "NOK bootstrap JWT CA missing keys",
			modifyConfig: func(cfg *InitConfig) {
				cfg.BootstrapResources = append(
					cfg.BootstrapResources,
					hostCA.Clone(),
					userCA.Clone(),
					invalidJWTCA.Clone(),
					dbServerCA.Clone(),
					dbClientCA.Clone(),
					osshCA.Clone(),
					samlCA.Clone(),
				)
			},
			assertError: require.Error,
		},
		{
			name: "NOK bootstrap Database CA missing keys",
			modifyConfig: func(cfg *InitConfig) {
				cfg.BootstrapResources = append(
					cfg.BootstrapResources,
					hostCA.Clone(),
					userCA.Clone(),
					jwtCA.Clone(),
					invalidDBServerCA.Clone(),
					osshCA.Clone(),
					samlCA.Clone(),
				)
			},
			assertError: require.Error,
		},
		{
			name: "NOK bootstrap Database Client CA missing keys",
			modifyConfig: func(cfg *InitConfig) {
				cfg.BootstrapResources = append(
					cfg.BootstrapResources,
					hostCA.Clone(),
					userCA.Clone(),
					jwtCA.Clone(),
					dbServerCA.Clone(),
					invalidDBClientCA.Clone(),
					osshCA.Clone(),
					samlCA.Clone(),
				)
			},
			assertError: require.Error,
		},
		{
			name: "NOK bootstrap OpenSSH CA missing keys",
			modifyConfig: func(cfg *InitConfig) {
				cfg.BootstrapResources = append(
					cfg.BootstrapResources,
					hostCA.Clone(),
					userCA.Clone(),
					jwtCA.Clone(),
					dbServerCA.Clone(),
					dbClientCA.Clone(),
					invalidOSSHCA.Clone(),
					samlCA.Clone(),
				)
			},
			assertError: require.Error,
		},
		{
			name: "NOK bootstrap SAML IdP CA missing keys",
			modifyConfig: func(cfg *InitConfig) {
				cfg.BootstrapResources = append(
					cfg.BootstrapResources,
					hostCA.Clone(),
					userCA.Clone(),
					jwtCA.Clone(),
					dbServerCA.Clone(),
					dbClientCA.Clone(),
					osshCA.Clone(),
					invalidSAMLCA.Clone(),
				)
			},
			assertError: require.Error,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := setupConfig(t)
			test.modifyConfig(&cfg)

			_, err := Init(context.Background(), cfg)
			test.assertError(t, err)
		})
	}
}

const (
	userYAML = `kind: user
version: v2
metadata:
  name: myuser
spec:
  roles: ["admin"]`
	tokenYAML = `kind: token
version: v2
metadata:
  name: github-token
  expires: "3000-01-01T00:00:00Z"
spec:
  roles: [Bot]
  join_method: github
  bot_name: github-demo
  github:
    allow:
      - repository: gravitational/example`
	roleYAML = `kind: role
version: v7
metadata:
  name: admin
  expires: "3000-01-01T00:00:00Z"
spec:
  allow:
    logins: ['admin']
    kubernetes_groups: ['edit']
    node_labels:
      '*': '*'
    kubernetes_labels:
      '*': '*'
    kubernetes_resources:
      - kind: '*'
        namespace: '*'
        name: '*'
        verbs: ['*']`
	lockYAML = `
kind: lock
version: v2
metadata:
  name: b1c785d8-8165-41fc-8dd6-252d534334d3
spec:
  created_at: "2023-11-07T18:44:35.361806Z"
  created_by: Admin
  target:
    user: myuser
`
	clusterNetworkingConfYAML = `
kind: cluster_networking_config
metadata:
  name: cluster-networking-config
spec:
  proxy_listener_mode: 1
`
	authPrefYAML = `
kind: cluster_auth_preference
metadata:
  name: cluster-auth-preference
spec:
  second_factor: off
  type: local
version: v2
`
)

func TestInit_ApplyOnStartup(t *testing.T) {
	t.Parallel()

	user := resourceFromYAML(t, userYAML).(types.User)
	token := resourceFromYAML(t, tokenYAML).(types.ProvisionToken)
	role := resourceFromYAML(t, roleYAML).(types.Role)
	lock := resourceFromYAML(t, lockYAML).(types.Lock)
	clusterNetworkingConfig := resourceFromYAML(t, clusterNetworkingConfYAML).(types.ClusterNetworkingConfig)
	authPref := resourceFromYAML(t, authPrefYAML).(types.AuthPreference)

	tests := []struct {
		name         string
		modifyConfig func(*InitConfig)
		assertError  require.ErrorAssertionFunc
	}{
		{
			name: "Apply unsupported resource",
			modifyConfig: func(cfg *InitConfig) {
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, lock)
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, user)
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, role)
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, token)
			},
			assertError: require.Error,
		},
		{
			name: "Apply ProvisionToken",
			modifyConfig: func(cfg *InitConfig) {
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, token)
			},
			assertError: require.NoError,
		},
		{
			name: "Apply User (invalid, missing role)",
			modifyConfig: func(cfg *InitConfig) {
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, user)
			},
			assertError: require.Error,
		},
		// We test both user+role and role+user to validate that ordering doesn't matter
		{
			name: "Apply User+Role",
			modifyConfig: func(cfg *InitConfig) {
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, user)
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, role)
			},
			assertError: require.NoError,
		},
		{
			name: "Apply Role+User",
			modifyConfig: func(cfg *InitConfig) {
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, user)
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, role)
			},
			assertError: require.NoError,
		},
		{
			name: "Apply Role",
			modifyConfig: func(cfg *InitConfig) {
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, role)
			},
			assertError: require.NoError,
		},
		{
			name: "Apply ClusterNetworkingConfig",
			modifyConfig: func(cfg *InitConfig) {
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, clusterNetworkingConfig)
			},
			assertError: require.NoError,
		},
		{
			name: "Apply AuthPreference",
			modifyConfig: func(cfg *InitConfig) {
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, authPref)
			},
			assertError: require.NoError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := setupConfig(t)
			test.modifyConfig(&cfg)

			_, err := Init(context.Background(), cfg)
			test.assertError(t, err)
		})
	}
}

func resourceFromYAML(t *testing.T, value string) types.Resource {
	t.Helper()

	ur := &services.UnknownResource{}
	err := kyaml.NewYAMLToJSONDecoder(strings.NewReader(value)).Decode(ur)
	require.NoError(t, err)

	resource, err := services.UnmarshalResource(ur.Kind, ur.Raw)
	require.NoError(t, err)
	return resource
}

func resourceDiff(res1, res2 types.Resource) string {
	return cmp.Diff(res1, res2,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
		cmpopts.EquateEmpty())
}

// TestSyncUpgadeWindowStartHour verifies the core logic of the upgrade window start
// hour behavior.
func TestSyncUpgradeWindowStartHour(t *testing.T) {
	ctx := context.Background()

	conf := setupConfig(t)
	authServer, err := Init(ctx, conf)
	require.NoError(t, err)
	t.Cleanup(func() { authServer.Close() })

	// no getter is registered, sync should fail
	require.Error(t, authServer.syncUpgradeWindowStartHour(ctx))

	// maintenance config does not exist yet
	cmc, err := authServer.GetClusterMaintenanceConfig(ctx)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, cmc)

	// set up fake getter
	var mu sync.Mutex
	var fakeHour int64
	var fakeError error
	authServer.SetUpgradeWindowStartHourGetter(func(ctx context.Context) (int64, error) {
		mu.Lock()
		defer mu.Unlock()
		return fakeHour, fakeError
	})

	// sync should now succeed
	require.NoError(t, authServer.syncUpgradeWindowStartHour(ctx))

	cmc, err = authServer.GetClusterMaintenanceConfig(ctx)
	require.NoError(t, err)

	agentWindow, ok := cmc.GetAgentUpgradeWindow()
	require.True(t, ok)

	require.Equal(t, uint32(0), agentWindow.UTCStartHour)

	// change the served hour
	mu.Lock()
	fakeHour = 16
	mu.Unlock()

	require.NoError(t, authServer.syncUpgradeWindowStartHour(ctx))

	cmc, err = authServer.GetClusterMaintenanceConfig(ctx)
	require.NoError(t, err)

	agentWindow, ok = cmc.GetAgentUpgradeWindow()
	require.True(t, ok)

	require.Equal(t, uint32(16), agentWindow.UTCStartHour)

	// set sync to fail with out of range hour
	mu.Lock()
	fakeHour = 36
	mu.Unlock()

	require.Error(t, authServer.syncUpgradeWindowStartHour(ctx))

	cmc, err = authServer.GetClusterMaintenanceConfig(ctx)
	require.NoError(t, err)

	agentWindow, ok = cmc.GetAgentUpgradeWindow()
	require.True(t, ok)

	// verify that the old hour value persists since the sync failed
	require.Equal(t, uint32(16), agentWindow.UTCStartHour)

	// set sync to fail with impossible int type-cast
	mu.Lock()
	fakeHour = math.MaxInt64
	mu.Unlock()

	require.Error(t, authServer.syncUpgradeWindowStartHour(ctx))

	cmc, err = authServer.GetClusterMaintenanceConfig(ctx)
	require.NoError(t, err)

	agentWindow, ok = cmc.GetAgentUpgradeWindow()
	require.True(t, ok)

	// verify that the old hour value persists since the sync failed
	require.Equal(t, uint32(16), agentWindow.UTCStartHour)

	mu.Lock()
	fakeHour = 18
	mu.Unlock()

	// sync should now succeed again
	require.NoError(t, authServer.syncUpgradeWindowStartHour(ctx))

	cmc, err = authServer.GetClusterMaintenanceConfig(ctx)
	require.NoError(t, err)

	agentWindow, ok = cmc.GetAgentUpgradeWindow()
	require.True(t, ok)

	// verify that we got the new hour value
	require.Equal(t, uint32(18), agentWindow.UTCStartHour)

	// set sync to fail with error
	mu.Lock()
	fakeHour = 12
	fakeError = fmt.Errorf("uh-oh")
	mu.Unlock()

	require.Error(t, authServer.syncUpgradeWindowStartHour(ctx))

	cmc, err = authServer.GetClusterMaintenanceConfig(ctx)
	require.NoError(t, err)

	agentWindow, ok = cmc.GetAgentUpgradeWindow()
	require.True(t, ok)

	// verify that the old hour value persists since the sync failed
	require.Equal(t, uint32(18), agentWindow.UTCStartHour)

	// recover and set hour to zero
	mu.Lock()
	fakeHour = 0
	fakeError = nil
	mu.Unlock()

	// sync should now succeed again
	require.NoError(t, authServer.syncUpgradeWindowStartHour(ctx))

	cmc, err = authServer.GetClusterMaintenanceConfig(ctx)
	require.NoError(t, err)

	agentWindow, ok = cmc.GetAgentUpgradeWindow()
	require.True(t, ok)

	// verify that we got the new hour value
	require.Equal(t, uint32(0), agentWindow.UTCStartHour)
}

// TestIdentityChecker verifies auth identity properly validates host
// certificates when connecting to an SSH server.
func TestIdentityChecker(t *testing.T) {
	ctx := context.Background()

	conf := setupConfig(t)
	authServer, err := Init(ctx, conf)
	require.NoError(t, err)
	t.Cleanup(func() { authServer.Close() })

	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentAuth,
			Client:    authServer,
		},
	})
	require.NoError(t, err)
	authServer.SetLockWatcher(lockWatcher)

	clusterName, err := authServer.GetDomainName()
	require.NoError(t, err)

	ca, err := authServer.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName,
	}, true)
	require.NoError(t, err)

	signers, err := sshutils.GetSigners(ca)
	require.NoError(t, err)
	require.Len(t, signers, 1)

	realCert, err := apisshutils.MakeRealHostCert(signers[0])
	require.NoError(t, err)

	spoofedCert, err := apisshutils.MakeSpoofedHostCert(signers[0])
	require.NoError(t, err)

	tests := []struct {
		desc string
		cert ssh.Signer
		err  bool
	}{
		{
			desc: "should be able to connect with real cert",
			cert: realCert,
			err:  false,
		},
		{
			desc: "should not be able to connect with spoofed cert",
			cert: spoofedCert,
			err:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			handler := sshutils.NewChanHandlerFunc(func(_ context.Context, ccx *sshutils.ConnectionContext, nch ssh.NewChannel) {
				ch, _, err := nch.Accept()
				require.NoError(t, err)
				require.NoError(t, ch.Close())
			})
			sshServer, err := sshutils.NewServer(
				"test",
				utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
				handler,
				sshutils.StaticHostSigners(test.cert),
				sshutils.AuthMethods{NoClient: true},
				sshutils.SetInsecureSkipHostValidation(),
			)
			require.NoError(t, err)
			t.Cleanup(func() { sshServer.Close() })
			require.NoError(t, sshServer.Start())

			identity, err := GenerateIdentity(authServer, state.IdentityID{
				Role:     types.RoleNode,
				HostUUID: uuid.New().String(),
				NodeName: "node-1",
			}, nil, nil)
			require.NoError(t, err)

			sshClientConfig, err := identity.SSHClientConfig(false)
			require.NoError(t, err)

			dialer := proxy.DialerFromEnvironment(sshServer.Addr())
			sconn, err := dialer.Dial(ctx, "tcp", sshServer.Addr(), sshClientConfig)
			if test.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NoError(t, sconn.Close())
			}
		})
	}
}

func TestInitCreatesCertsIfMissing(t *testing.T) {
	ctx := context.Background()
	conf := setupConfig(t)
	auth, err := Init(ctx, conf)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = auth.Close()
		require.NoError(t, err)
	})

	for _, caType := range types.CertAuthTypes {
		cert, err := auth.GetCertAuthorities(ctx, caType, false)
		require.NoError(t, err)
		require.Len(t, cert, 1)
	}
}

func TestTeleportProcessAuthVersionUpgradeCheck(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	tests := []struct {
		name            string
		initialVersion  string
		expectedVersion string
		expectError     bool
		skipCheck       bool
	}{
		{
			name:            "first-launch",
			initialVersion:  "",
			expectedVersion: teleport.Version,
			expectError:     false,
		},
		{
			name:            "old-version-upgrade",
			initialVersion:  fmt.Sprintf("%d.0.0", teleport.SemVersion.Major-1),
			expectedVersion: teleport.Version,
			expectError:     false,
		},
		{
			name:            "major-upgrade-fail",
			initialVersion:  fmt.Sprintf("%d.0.0", teleport.SemVersion.Major-2),
			expectedVersion: fmt.Sprintf("%d.0.0", teleport.SemVersion.Major-2),
			expectError:     true,
		},
		{
			name:            "major-upgrade-with-dev-skip-check",
			initialVersion:  fmt.Sprintf("%d.0.0", teleport.SemVersion.Major-2),
			expectedVersion: fmt.Sprintf("%d.0.0", teleport.SemVersion.Major-2),
			expectError:     false,
			skipCheck:       true,
		},
		{
			name:            "major-downgrade-fail",
			initialVersion:  fmt.Sprintf("%d.0.0", teleport.SemVersion.Major+2),
			expectedVersion: fmt.Sprintf("%d.0.0", teleport.SemVersion.Major+2),
			expectError:     true,
		},
		{
			name:            "major-downgrade-with-dev-skip-check",
			initialVersion:  fmt.Sprintf("%d.0.0", teleport.SemVersion.Major+2),
			expectedVersion: fmt.Sprintf("%d.0.0", teleport.SemVersion.Major+2),
			expectError:     false,
			skipCheck:       true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			authCfg := setupConfig(t)

			if test.initialVersion != "" {
				err := authCfg.VersionStorage.WriteTeleportVersion(ctx, semver.New(test.initialVersion))
				require.NoError(t, err)
			}
			if test.skipCheck {
				t.Setenv(skipVersionUpgradeCheckEnv, "yes")
			}

			_, err := Init(ctx, authCfg)
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			lastKnownVersion, err := authCfg.VersionStorage.GetTeleportVersion(ctx)
			require.NoError(t, err)
			require.Equal(t, test.expectedVersion, lastKnownVersion.String())
		})
	}
}

type mockDatabaseObjectImportRules struct {
	services.DatabaseObjectImportRules
	listRules []*dbobjectimportrulev1.DatabaseObjectImportRule
	created   *dbobjectimportrulev1.DatabaseObjectImportRule
	upserted  *dbobjectimportrulev1.DatabaseObjectImportRule
}

func (m *mockDatabaseObjectImportRules) ListDatabaseObjectImportRules(context.Context, int, string) ([]*dbobjectimportrulev1.DatabaseObjectImportRule, string, error) {
	return m.listRules, "", nil
}
func (m *mockDatabaseObjectImportRules) CreateDatabaseObjectImportRule(ctx context.Context, rule *dbobjectimportrulev1.DatabaseObjectImportRule) (*dbobjectimportrulev1.DatabaseObjectImportRule, error) {
	m.created = rule
	return rule, nil
}
func (m *mockDatabaseObjectImportRules) UpsertDatabaseObjectImportRule(ctx context.Context, rule *dbobjectimportrulev1.DatabaseObjectImportRule) (*dbobjectimportrulev1.DatabaseObjectImportRule, error) {
	m.upserted = rule
	return rule, nil
}

func Test_createPresetDatabaseObjectImportRule(t *testing.T) {
	presetRule := databaseobjectimportrule.NewPresetImportAllObjectsRule()
	require.NotNil(t, presetRule)

	customRule, err := databaseobjectimportrule.NewDatabaseObjectImportRule("dev_rule", &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
		Priority:       100,
		DatabaseLabels: label.FromMap(map[string][]string{"env": {"dev"}}),
		Mappings: []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{{
			Match: &dbobjectimportrulev1.DatabaseObjectImportMatch{
				TableNames: []string{"*"},
			},
			AddLabels: map[string]string{
				"env": "dev",
			},
			Scope: &dbobjectimportrulev1.DatabaseObjectImportScope{
				SchemaNames: []string{"public"},
			},
		}},
	})
	require.NoError(t, err)

	oldPresetRule, err := databaseobjectimportrule.NewDatabaseObjectImportRule("import_all_objects", &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
		DatabaseLabels: label.FromMap(map[string][]string{"*": {"*"}}),
		Mappings: []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{
			{
				Match:     &dbobjectimportrulev1.DatabaseObjectImportMatch{TableNames: []string{"*"}},
				AddLabels: map[string]string{"kind": "table"},
			},
			{
				Match:     &dbobjectimportrulev1.DatabaseObjectImportMatch{ViewNames: []string{"*"}},
				AddLabels: map[string]string{"kind": "view"},
			},
			{
				Match:     &dbobjectimportrulev1.DatabaseObjectImportMatch{ProcedureNames: []string{"*"}},
				AddLabels: map[string]string{"kind": "procedure"},
			},
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name          string
		existingRules []*dbobjectimportrulev1.DatabaseObjectImportRule
		expectCreate  *dbobjectimportrulev1.DatabaseObjectImportRule
		expectUpsert  *dbobjectimportrulev1.DatabaseObjectImportRule
	}{
		{
			name:         "create preset in new cluster",
			expectCreate: presetRule,
		},
		{
			name:          "no action with custom rule",
			existingRules: []*dbobjectimportrulev1.DatabaseObjectImportRule{customRule},
		},
		{
			name:          "no action with old preset and custom rule",
			existingRules: []*dbobjectimportrulev1.DatabaseObjectImportRule{oldPresetRule, customRule},
		},
		{
			name:          "no action with preset rule",
			existingRules: []*dbobjectimportrulev1.DatabaseObjectImportRule{presetRule},
		},
		{
			name:          "migrate old preset to new",
			existingRules: []*dbobjectimportrulev1.DatabaseObjectImportRule{oldPresetRule},
			expectUpsert:  presetRule,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			m := &mockDatabaseObjectImportRules{
				listRules: test.existingRules,
			}

			err := createPresetDatabaseObjectImportRule(context.Background(), m)
			require.NoError(t, err)
			require.True(t, proto.Equal(test.expectCreate, m.created))
			require.True(t, proto.Equal(test.expectUpsert, m.upserted))
		})
	}
}

// TestInitWithAutoUpdateBootstrap verifies that auth init support bootstrapping `AutoUpdateConfig` and
// `AutoUpdateVersion` resources as well as unmarshalling them from yaml configuration.
func TestInitWithAutoUpdateBootstrap(t *testing.T) {
	t.Parallel()

	const autoUpdateConfigYAML = `kind: autoupdate_config
metadata:
  name: autoupdate-config
spec:
  tools:
    mode: enabled
version: v1`
	const autoUpdateVersionYAML = `kind: autoupdate_version
metadata:
  name: autoupdate-version
spec:
  tools:
    target_version: 1.2.3
version: v1`

	ctx := context.Background()

	cfg := setupConfig(t)
	cfg.BootstrapResources = []types.Resource{
		resourceFromYAML(t, autoUpdateConfigYAML),
		resourceFromYAML(t, autoUpdateVersionYAML),
	}

	auth, err := Init(ctx, cfg)
	require.NoError(t, err)

	config, err := auth.GetAutoUpdateConfig(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "enabled", config.GetSpec().GetTools().GetMode())

	version, err := auth.GetAutoUpdateVersion(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3", version.GetSpec().GetTools().GetTargetVersion())
}
