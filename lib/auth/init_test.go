/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/proxy"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// TestReadIdentity makes parses identity from private key and certificate
// and checks that all parameters are valid
func TestReadIdentity(t *testing.T) {
	clock := clockwork.NewFakeClock()
	a := testauthority.NewWithClock(clock)
	priv, pub, err := a.GenerateKeyPair("")
	require.NoError(t, err)
	caSigner, err := ssh.ParsePrivateKey(priv)
	require.NoError(t, err)

	cert, err := a.GenerateHostCert(services.HostCertParams{
		CASigner:      caSigner,
		CASigningAlg:  defaults.CASignatureAlgorithm,
		PublicHostKey: pub,
		HostID:        "id1",
		NodeName:      "node-name",
		ClusterName:   "example.com",
		Role:          types.RoleNode,
		TTL:           0,
	})
	require.NoError(t, err)

	id, err := ReadSSHIdentityFromKeyPair(priv, cert)
	require.NoError(t, err)
	require.Equal(t, id.ClusterName, "example.com")
	require.Equal(t, id.ID, IdentityID{HostUUID: "id1.example.com", Role: types.RoleNode})
	require.Equal(t, id.CertBytes, cert)
	require.Equal(t, id.KeyBytes, priv)

	// test TTL by converting the generated cert to text -> back and making sure ExpireAfter is valid
	ttl := 10 * time.Second
	expiryDate := clock.Now().Add(ttl)
	bytes, err := a.GenerateHostCert(services.HostCertParams{
		CASigner:      caSigner,
		CASigningAlg:  defaults.CASignatureAlgorithm,
		PublicHostKey: pub,
		HostID:        "id1",
		NodeName:      "node-name",
		ClusterName:   "example.com",
		Role:          types.RoleNode,
		TTL:           ttl,
	})
	require.NoError(t, err)
	copy, err := apisshutils.ParseCertificate(bytes)
	require.NoError(t, err)
	require.Equal(t, uint64(expiryDate.Unix()), copy.ValidBefore)
}

func TestBadIdentity(t *testing.T) {
	a := testauthority.New()
	priv, pub, err := a.GenerateKeyPair("")
	require.NoError(t, err)
	caSigner, err := ssh.ParsePrivateKey(priv)
	require.NoError(t, err)

	// bad cert type
	_, err = ReadSSHIdentityFromKeyPair(priv, pub)
	require.IsType(t, trace.BadParameter(""), err)

	// missing authority domain
	cert, err := a.GenerateHostCert(services.HostCertParams{
		CASigner:      caSigner,
		CASigningAlg:  defaults.CASignatureAlgorithm,
		PublicHostKey: pub,
		HostID:        "id2",
		NodeName:      "",
		ClusterName:   "",
		Role:          types.RoleNode,
		TTL:           0,
	})
	require.NoError(t, err)

	_, err = ReadSSHIdentityFromKeyPair(priv, cert)
	require.IsType(t, trace.BadParameter(""), err)

	// missing host uuid
	cert, err = a.GenerateHostCert(services.HostCertParams{
		CASigner:      caSigner,
		CASigningAlg:  defaults.CASignatureAlgorithm,
		PublicHostKey: pub,
		HostID:        "example.com",
		NodeName:      "",
		ClusterName:   "",
		Role:          types.RoleNode,
		TTL:           0,
	})
	require.NoError(t, err)

	_, err = ReadSSHIdentityFromKeyPair(priv, cert)
	require.IsType(t, trace.BadParameter(""), err)

	// unrecognized role
	cert, err = a.GenerateHostCert(services.HostCertParams{
		CASigner:      caSigner,
		CASigningAlg:  defaults.CASignatureAlgorithm,
		PublicHostKey: pub,
		HostID:        "example.com",
		NodeName:      "",
		ClusterName:   "id1",
		Role:          "bad role",
		TTL:           0,
	})
	require.NoError(t, err)

	_, err = ReadSSHIdentityFromKeyPair(priv, cert)
	require.IsType(t, trace.BadParameter(""), err)
}

type testDynamicallyConfigurableParams struct {
	withDefaults, withConfigFile, withAnotherConfigFile func(*testing.T, *InitConfig) types.ResourceWithOrigin
	setDynamic                                          func(*testing.T, *Server)
	getStored                                           func(*testing.T, *Server) types.ResourceWithOrigin
}

func testDynamicallyConfigurable(t *testing.T, p testDynamicallyConfigurableParams) {
	initAuthServer := func(t *testing.T, conf InitConfig) *Server {
		authServer, err := Init(conf)
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

		// Verify the stored resource is now labelled as originating from defaults.
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
				Type: constants.OIDC,
			})
			require.NoError(t, err)
			conf.AuthPreference = fromConfigFile
			return conf.AuthPreference
		},
		withAnotherConfigFile: func(t *testing.T, conf *InitConfig) types.ResourceWithOrigin {
			conf.AuthPreference = newU2FAuthPreferenceFromConfigFile(t)
			return conf.AuthPreference
		},
		setDynamic: func(t *testing.T, authServer *Server) {
			dynamically, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
				SecondFactor: constants.SecondFactorOff,
			})
			require.NoError(t, err)
			err = authServer.SetAuthPreference(ctx, dynamically)
			require.NoError(t, err)
		},
		getStored: func(t *testing.T, authServer *Server) types.ResourceWithOrigin {
			authPref, err := authServer.GetAuthPreference(ctx)
			require.NoError(t, err)
			return authPref
		},
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
			err = authServer.SetClusterNetworkingConfig(ctx, dynamically)
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
			err = authServer.SetSessionRecordingConfig(ctx, dynamically)
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
	authServer, err := Init(conf)
	require.NoError(t, err)
	defer authServer.Close()

	cc, err := authServer.GetClusterName()
	require.NoError(t, err)
	clusterID := cc.GetClusterID()
	require.NotEqual(t, clusterID, "")

	// do it again and make sure cluster ID hasn't changed
	authServer, err = Init(conf)
	require.NoError(t, err)
	defer authServer.Close()

	cc, err = authServer.GetClusterName()
	require.NoError(t, err)
	require.Equal(t, cc.GetClusterID(), clusterID)
}

// TestClusterName ensures that a cluster can not be renamed.
func TestClusterName(t *testing.T) {
	conf := setupConfig(t)
	authServer, err := Init(conf)
	require.NoError(t, err)
	defer authServer.Close()

	// Start the auth server with a different cluster name. The auth server
	// should start, but with the original name.
	newConfig := conf
	newConfig.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "dev.localhost",
	})
	require.NoError(t, err)
	authServer, err = Init(newConfig)
	require.NoError(t, err)
	defer authServer.Close()

	cn, err := authServer.GetClusterName()
	require.NoError(t, err)
	require.NotEqual(t, newConfig.ClusterName.GetClusterName(), cn.GetClusterName())
	require.Equal(t, conf.ClusterName.GetClusterName(), cn.GetClusterName())
}

func TestCASigningAlg(t *testing.T) {
	verifyCAs := func(auth *Server, alg string) {
		hostCAs, err := auth.GetCertAuthorities(types.HostCA, false)
		require.NoError(t, err)
		for _, ca := range hostCAs {
			require.Equal(t, sshutils.GetSigningAlgName(ca), alg)
		}
		userCAs, err := auth.GetCertAuthorities(types.UserCA, false)
		require.NoError(t, err)
		for _, ca := range userCAs {
			require.Equal(t, sshutils.GetSigningAlgName(ca), alg)
		}
		dbCAs, err := auth.GetCertAuthorities(types.DatabaseCA, false)
		require.NoError(t, err)
		for _, ca := range dbCAs {
			require.Equal(t, sshutils.GetSigningAlgName(ca), alg)
		}
	}

	// Start a new server without specifying a signing alg.
	conf := setupConfig(t)
	auth, err := Init(conf)
	require.NoError(t, err)
	defer auth.Close()
	verifyCAs(auth, ssh.SigAlgoRSASHA2512)

	require.NoError(t, auth.Close())

	// Reset the auth server state.
	conf.Backend, err = lite.New(context.TODO(), backend.Params{"path": t.TempDir()})
	require.NoError(t, err)
	conf.DataDir = t.TempDir()

	// Start a new server with non-default signing alg.
	signingAlg := ssh.SigAlgoRSA
	conf.CASigningAlg = &signingAlg
	auth, err = Init(conf)
	require.NoError(t, err)
	defer auth.Close()
	verifyCAs(auth, ssh.SigAlgoRSA)

	// Start again, using a different alg. This should not change the existing
	// CA.
	signingAlg = ssh.SigAlgoRSASHA2256
	auth, err = Init(conf)
	require.NoError(t, err)
	verifyCAs(auth, ssh.SigAlgoRSA)
}

// TestPresets tests behavior of presets
func TestPresets(t *testing.T) {
	ctx := context.Background()
	roles := []types.Role{
		services.NewPresetEditorRole(),
		services.NewPresetAccessRole(),
		services.NewPresetAuditorRole()}

	t.Run("EmptyCluster", func(t *testing.T) {
		as := newTestAuthServer(ctx, t)
		clock := clockwork.NewFakeClock()
		as.SetClock(clock)

		err := createPresets(as)
		require.NoError(t, err)

		// Second call should not fail
		err = createPresets(as)
		require.NoError(t, err)

		// Presets were created
		for _, role := range roles {
			_, err := as.GetRole(ctx, role.GetName())
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
		err := as.CreateRole(access)
		require.NoError(t, err)

		err = createPresets(as)
		require.NoError(t, err)

		// Presets were created
		for _, role := range roles {
			_, err := as.GetRole(ctx, role.GetName())
			require.NoError(t, err)
		}

		out, err := as.GetRole(ctx, access.GetName())
		require.NoError(t, err)
		require.Equal(t, access.GetLogins(types.Allow), out.GetLogins(types.Allow))
	})
}

func setupConfig(t *testing.T) InitConfig {
	tempDir := t.TempDir()

	bk, err := lite.New(context.TODO(), backend.Params{"path": tempDir})
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	return InitConfig{
		DataDir:                 tempDir,
		HostUUID:                "00000000-0000-0000-0000-000000000000",
		NodeName:                "foo",
		Backend:                 bk,
		Authority:               testauthority.New(),
		ClusterAuditConfig:      types.DefaultClusterAuditConfig(),
		ClusterNetworkingConfig: types.DefaultClusterNetworkingConfig(),
		SessionRecordingConfig:  types.DefaultSessionRecordingConfig(),
		ClusterName:             clusterName,
		StaticTokens:            types.DefaultStaticTokens(),
		AuthPreference:          types.DefaultAuthPreference(),
		SkipPeriodicOperations:  true,
	}
}

func newU2FAuthPreferenceFromConfigFile(t *testing.T) types.AuthPreference {
	ap, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorU2F,
		U2F: &types.U2F{
			AppID:  "foo",
			Facets: []string{"bar", "baz"},
		},
	})
	require.NoError(t, err)
	return ap
}

func TestMigrateCertAuthorities(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	as := newTestAuthServer(ctx, t)
	clock := clockwork.NewFakeClock()
	as.SetClock(clock)

	for _, spec := range []types.CertAuthoritySpecV2{
		{
			Type:         types.HostCA,
			ClusterName:  "localhost",
			CheckingKeys: [][]byte{[]byte(fixtures.SSHCAPublicKey)},
			SigningKeys:  [][]byte{[]byte(fixtures.SSHCAPrivateKey)},
			TLSKeyPairs:  []types.TLSKeyPair{{Cert: []byte(fixtures.TLSCACertPEM), Key: []byte(fixtures.TLSCAKeyPEM)}},
			Rotation:     nil, // Rotation was never performed.
		}, //TODO(JN) DB CA
		{
			Type:         types.UserCA,
			ClusterName:  "localhost",
			CheckingKeys: [][]byte{[]byte(fixtures.SSHCAPublicKey)},
			SigningKeys:  [][]byte{[]byte(fixtures.SSHCAPrivateKey)},
			TLSKeyPairs:  []types.TLSKeyPair{{Cert: []byte(fixtures.TLSCACertPEM), Key: []byte(fixtures.TLSCAKeyPEM)}},
			Rotation:     &types.Rotation{State: types.RotationStateStandby},
		},
		{
			Type:        types.JWTSigner,
			ClusterName: "localhost",
			JWTKeyPairs: []types.JWTKeyPair{{PublicKey: []byte(fixtures.JWTSignerPublicKey), PrivateKey: []byte(fixtures.JWTSignerPrivateKey)}},
			Rotation:    &types.Rotation{State: types.RotationStateStandby},
		},
	} {
		t.Run(fmt.Sprintf("create %v CA", spec.Type), func(t *testing.T) {
			ca, err := types.NewCertAuthority(spec)
			require.NoError(t, err)
			// Do NOT use services.MarshalCertAuthority to keep all fields as-is.
			enc, err := utils.FastMarshal(ca)
			require.NoError(t, err)

			_, err = as.bk.Put(ctx, backend.Item{
				Key:   backend.Key("authorities", string(ca.GetType()), ca.GetName()),
				Value: enc,
			})
			require.NoError(t, err)
		})
	}

	err := migrateCertAuthorities(ctx, as)
	require.NoError(t, err)

	var caSpecs []types.CertAuthoritySpecV2
	for _, typ := range []types.CertAuthType{types.HostCA, types.UserCA, types.JWTSigner} {
		t.Run(fmt.Sprintf("verify %v CA", typ), func(t *testing.T) {
			cas, err := as.GetCertAuthorities(typ, true)
			require.NoError(t, err)
			require.Len(t, cas, 1)
			caSpecs = append(caSpecs, cas[0].(*types.CertAuthorityV2).Spec)
		})
	}
	require.Empty(t, cmp.Diff(caSpecs, []types.CertAuthoritySpecV2{
		{
			Type:        types.HostCA,
			ClusterName: "localhost",
			ActiveKeys: types.CAKeySet{
				SSH: []*types.SSHKeyPair{{
					PrivateKey: []byte(fixtures.SSHCAPrivateKey),
					PublicKey:  []byte(fixtures.SSHCAPublicKey),
				}},
				TLS: []*types.TLSKeyPair{{Cert: []byte(fixtures.TLSCACertPEM), Key: []byte(fixtures.TLSCAKeyPEM)}},
			},
			CheckingKeys: [][]byte{[]byte(fixtures.SSHCAPublicKey)},
			SigningKeys:  [][]byte{[]byte(fixtures.SSHCAPrivateKey)},
			TLSKeyPairs:  []types.TLSKeyPair{{Cert: []byte(fixtures.TLSCACertPEM), Key: []byte(fixtures.TLSCAKeyPEM)}},
			Rotation:     nil,
		},
		{
			Type:        types.UserCA,
			ClusterName: "localhost",
			ActiveKeys: types.CAKeySet{
				SSH: []*types.SSHKeyPair{{
					PrivateKey: []byte(fixtures.SSHCAPrivateKey),
					PublicKey:  []byte(fixtures.SSHCAPublicKey),
				}},
				TLS: []*types.TLSKeyPair{{Cert: []byte(fixtures.TLSCACertPEM), Key: []byte(fixtures.TLSCAKeyPEM)}},
			},
			CheckingKeys: [][]byte{[]byte(fixtures.SSHCAPublicKey)},
			SigningKeys:  [][]byte{[]byte(fixtures.SSHCAPrivateKey)},
			TLSKeyPairs:  []types.TLSKeyPair{{Cert: []byte(fixtures.TLSCACertPEM), Key: []byte(fixtures.TLSCAKeyPEM)}},
			Rotation:     &types.Rotation{State: types.RotationStateStandby},
		},
		{
			Type:        types.JWTSigner,
			ClusterName: "localhost",
			ActiveKeys: types.CAKeySet{
				JWT: []*types.JWTKeyPair{{PublicKey: []byte(fixtures.JWTSignerPublicKey), PrivateKey: []byte(fixtures.JWTSignerPrivateKey)}},
			},
			JWTKeyPairs: []types.JWTKeyPair{{PublicKey: []byte(fixtures.JWTSignerPublicKey), PrivateKey: []byte(fixtures.JWTSignerPrivateKey)}},
			Rotation:    &types.Rotation{State: types.RotationStateStandby},
		},
	}))
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
)

func TestInit_bootstrap(t *testing.T) {
	t.Parallel()

	hostCA := resourceFromYAML(t, hostCAYAML).(types.CertAuthority)
	userCA := resourceFromYAML(t, userCAYAML).(types.CertAuthority)
	jwtCA := resourceFromYAML(t, jwtCAYAML).(types.CertAuthority)

	invalidHostCA := resourceFromYAML(t, hostCAYAML).(types.CertAuthority)
	invalidHostCA.(*types.CertAuthorityV2).Spec.ActiveKeys.SSH = nil
	invalidUserCA := resourceFromYAML(t, userCAYAML).(types.CertAuthority)
	invalidUserCA.(*types.CertAuthorityV2).Spec.ActiveKeys.SSH = nil
	invalidJWTCA := resourceFromYAML(t, jwtCAYAML).(types.CertAuthority)
	invalidJWTCA.(*types.CertAuthorityV2).Spec.ActiveKeys.JWT = nil
	invalidJWTCA.(*types.CertAuthorityV2).Spec.JWTKeyPairs = nil

	tests := []struct {
		name         string
		modifyConfig func(*InitConfig)
		wantErr      bool
	}{
		{
			// Issue https://github.com/gravitational/teleport/issues/7853.
			name: "OK bootstrap CAs",
			modifyConfig: func(cfg *InitConfig) {
				cfg.Resources = append(cfg.Resources, hostCA.Clone(), userCA.Clone(), jwtCA.Clone())
			},
		},
		{
			name: "NOK bootstrap Host CA missing keys",
			modifyConfig: func(cfg *InitConfig) {
				cfg.Resources = append(cfg.Resources, invalidHostCA.Clone(), userCA.Clone(), jwtCA.Clone())
			},
			wantErr: true,
		},
		{
			name: "NOK bootstrap User CA missing keys",
			modifyConfig: func(cfg *InitConfig) {
				cfg.Resources = append(cfg.Resources, hostCA.Clone(), invalidUserCA.Clone(), jwtCA.Clone())
			},
			wantErr: true,
		},
		{
			name: "NOK bootstrap JWT CA missing keys",
			modifyConfig: func(cfg *InitConfig) {
				cfg.Resources = append(cfg.Resources, hostCA.Clone(), userCA.Clone(), invalidJWTCA.Clone())
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			cfg := setupConfig(t)
			test.modifyConfig(&cfg)

			_, err := Init(cfg)
			hasErr := err != nil
			require.Equal(t, test.wantErr, hasErr, err)
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace"),
		cmpopts.EquateEmpty())
}

// TestIdentityChecker verifies auth identity properly validates host
// certificates when connecting to an SSH server.
func TestIdentityChecker(t *testing.T) {
	ctx := context.Background()

	conf := setupConfig(t)
	authServer, err := Init(conf)
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

	ca, err := authServer.GetCertAuthority(types.CertAuthID{
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
				[]ssh.Signer{test.cert},
				sshutils.AuthMethods{NoClient: true},
				sshutils.SetInsecureSkipHostValidation(),
			)
			require.NoError(t, err)
			t.Cleanup(func() { sshServer.Close() })
			require.NoError(t, sshServer.Start())

			identity, err := GenerateIdentity(authServer, IdentityID{
				Role:     types.RoleNode,
				HostUUID: uuid.New().String(),
				NodeName: "node-1",
			}, nil, nil)
			require.NoError(t, err)

			sshClientConfig, err := identity.SSHClientConfig(false)
			require.NoError(t, err)

			dialer := proxy.DialerFromEnvironment(sshServer.Addr())
			sconn, err := dialer.Dial("tcp", sshServer.Addr(), sshClientConfig)
			if test.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NoError(t, sconn.Close())
			}
		})
	}
}
