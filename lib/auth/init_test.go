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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/trace"
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
		Roles:         types.SystemRoles{types.RoleNode},
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
		Roles:         types.SystemRoles{types.RoleNode},
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
		Roles:         types.SystemRoles{types.RoleNode},
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
		Roles:         types.SystemRoles{types.RoleNode},
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
		Roles:         types.SystemRoles{types.SystemRole("bad role")},
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

	resourceDiff := func(res1, res2 types.Resource) string {
		return cmp.Diff(res1, res2,
			cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace"),
			cmpopts.EquateEmpty())
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

func TestMigrateMFADevices(t *testing.T) {
	ctx := context.Background()
	as := newTestAuthServer(ctx, t)
	clock := clockwork.NewFakeClock()
	as.SetClock(clock)

	// Fake credentials and MFA secrets for migration.
	fakePasswordHash := []byte(`$2a$10$Yy.e6BmS2SrGbBDsyDLVkOANZmvjjMR890nUGSXFJHBXWzxe7T44m`)
	totpKey := "totp-key"
	u2fPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	u2fPubKey := u2fPrivKey.PublicKey
	u2fPubKeyBin, err := x509.MarshalPKIXPublicKey(&u2fPubKey)
	require.NoError(t, err)
	u2fKeyHandle := []byte("dummy handle")

	// Create un-migrated users.
	for name, localAuth := range map[string]*backend.Item{
		"no-mfa-user": nil,
		// Insert MFA data in the legacy format by manually writing to the
		// backend. All the code for writing these in lib/services/local was
		// removed.
		"totp-user": {
			Key:   []byte("/web/users/totp-user/totp"),
			Value: []byte(totpKey),
		},
		"u2f-user": {
			Key: []byte("/web/users/u2f-user/u2fregistration"),
			Value: []byte(fmt.Sprintf(`{"keyhandle":%q,"marshalled_pubkey":%q}`,
				base64.StdEncoding.EncodeToString(u2fKeyHandle),
				base64.StdEncoding.EncodeToString(u2fPubKeyBin),
			)),
		},
	} {
		u, err := types.NewUser(name)
		require.NoError(t, err)
		// Set a fake but valid bcrypt password hash.
		u.SetLocalAuth(&types.LocalAuthSecrets{PasswordHash: fakePasswordHash})
		err = as.CreateUser(ctx, u)
		require.NoError(t, err)

		if localAuth != nil {
			_, err = as.bk.Put(ctx, *localAuth)
			require.NoError(t, err)
		}
	}

	// Run the migration.
	err = migrateMFADevices(ctx, as)
	require.NoError(t, err)

	// Generate expected users with migrated MFA.
	requireNewDevice := func(d *types.MFADevice, err error) []*types.MFADevice {
		require.NoError(t, err)
		return []*types.MFADevice{d}
	}
	wantUsers := []types.User{
		newUserWithAuth(t, "no-mfa-user", &types.LocalAuthSecrets{PasswordHash: fakePasswordHash}),
		newUserWithAuth(t, "totp-user", &types.LocalAuthSecrets{
			PasswordHash: fakePasswordHash,
			TOTPKey:      totpKey,
			MFA:          requireNewDevice(services.NewTOTPDevice("totp", totpKey, clock.Now())),
		}),
		newUserWithAuth(t, "u2f-user", &types.LocalAuthSecrets{
			PasswordHash: fakePasswordHash,
			U2FRegistration: &types.U2FRegistrationData{
				KeyHandle: u2fKeyHandle,
				PubKey:    u2fPubKeyBin,
			},
			MFA: requireNewDevice(u2f.NewDevice("u2f", &u2f.Registration{
				KeyHandle: u2fKeyHandle,
				PubKey:    u2fPubKey,
			}, clock.Now())),
		}),
	}
	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(types.UserSpecV2{}, "CreatedBy"),
		cmpopts.IgnoreFields(types.MFADevice{}, "Id"),
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		cmpopts.SortSlices(func(a, b types.User) bool { return a.GetName() < b.GetName() }),
	}

	// Check the actual users from the backend.
	users, err := as.GetUsers(true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(users, wantUsers, cmpOpts...))

	// A second migration should be a noop.
	err = migrateMFADevices(ctx, as)
	require.NoError(t, err)

	users, err = as.GetUsers(true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(users, wantUsers, cmpOpts...))
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

		err := createPresets(ctx, as)
		require.NoError(t, err)

		// Second call should not fail
		err = createPresets(ctx, as)
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

		err = createPresets(ctx, as)
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

// TestMigrateOSS tests migration of OSS users, github connectors
// and trusted clusters
func TestMigrateOSS(t *testing.T) {
	ctx := context.Background()

	t.Run("EmptyCluster", func(t *testing.T) {
		as := newTestAuthServer(ctx, t)
		clock := clockwork.NewFakeClock()
		as.SetClock(clock)

		// create non-migrated admin role
		err := as.CreateRole(services.NewAdminRole())
		require.NoError(t, err)

		err = migrateOSS(ctx, as)
		require.NoError(t, err)

		// Second call should not fail
		err = migrateOSS(ctx, as)
		require.NoError(t, err)

		// OSS user role was updated
		role, err := as.GetRole(ctx, teleport.AdminRoleName)
		require.NoError(t, err)
		require.Equal(t, types.True, role.GetMetadata().Labels[teleport.OSSMigratedV6])
	})

	t.Run("User", func(t *testing.T) {
		as := newTestAuthServer(ctx, t)
		clock := clockwork.NewFakeClock()
		as.SetClock(clock)

		// create non-migrated admin role to kick off migration
		err := as.CreateRole(services.NewAdminRole())
		require.NoError(t, err)

		user, _, err := CreateUserAndRole(as, "alice", []string{"alice"})
		require.NoError(t, err)

		err = migrateOSS(ctx, as)
		require.NoError(t, err)

		out, err := as.GetUser(user.GetName(), false)
		require.NoError(t, err)
		require.Equal(t, []string{teleport.AdminRoleName}, out.GetRoles())
		require.Equal(t, types.True, out.GetMetadata().Labels[teleport.OSSMigratedV6])

		err = migrateOSS(ctx, as)
		require.NoError(t, err)
	})

	t.Run("TrustedCluster", func(t *testing.T) {
		clusterName := "test.localhost"
		as := newTestAuthServer(ctx, t, clusterName)
		clock := clockwork.NewFakeClock()
		as.SetClock(clock)

		// create non-migrated admin role to kick off migration
		err := as.CreateRole(services.NewAdminRole())
		require.NoError(t, err)

		foo, err := types.NewTrustedCluster("foo", types.TrustedClusterSpecV2{
			Enabled:              false,
			Token:                "qux",
			ProxyAddress:         "quux",
			ReverseTunnelAddress: "quuz",
		})
		require.NoError(t, err)

		value, err := services.MarshalTrustedCluster(foo)
		require.NoError(t, err)

		_, err = as.bk.Put(ctx, backend.Item{
			Key:   []byte("/trustedclusters/foo"),
			Value: value,
		})
		require.NoError(t, err)

		for _, name := range []string{clusterName, foo.GetName()} {
			for _, catype := range []types.CertAuthType{types.UserCA, types.HostCA} {
				causer := suite.NewTestCA(catype, name)
				err = as.UpsertCertAuthority(causer)
				require.NoError(t, err)
			}
		}

		err = migrateOSS(ctx, as)
		require.NoError(t, err)

		out, err := as.GetTrustedCluster(ctx, foo.GetName())
		require.NoError(t, err)
		mapping := types.RoleMap{{Remote: teleport.AdminRoleName, Local: []string{teleport.AdminRoleName}}}
		require.Equal(t, mapping, out.GetRoleMap())

		for _, catype := range []types.CertAuthType{types.UserCA, types.HostCA} {
			ca, err := as.GetCertAuthority(types.CertAuthID{Type: catype, DomainName: foo.GetName()}, true)
			require.NoError(t, err)
			require.Equal(t, mapping, ca.GetRoleMap())
			require.Equal(t, types.True, ca.GetMetadata().Labels[teleport.OSSMigratedV6])
		}

		// root cluster CA are not updated
		for _, catype := range []types.CertAuthType{types.UserCA, types.HostCA} {
			ca, err := as.GetCertAuthority(types.CertAuthID{Type: catype, DomainName: clusterName}, true)
			require.NoError(t, err)
			_, found := ca.GetMetadata().Labels[teleport.OSSMigratedV6]
			require.False(t, found)
		}

		err = migrateOSS(ctx, as)
		require.NoError(t, err)
	})

	t.Run("GithubConnector", func(t *testing.T) {
		as := newTestAuthServer(ctx, t)
		clock := clockwork.NewFakeClock()
		as.SetClock(clock)

		// create non-migrated admin role to kick off migration
		err := as.CreateRole(services.NewAdminRole())
		require.NoError(t, err)

		connector, err := types.NewGithubConnector("github", types.GithubConnectorSpecV3{
			ClientID:     "aaa",
			ClientSecret: "bbb",
			RedirectURL:  "https://localhost:3080/v1/webapi/github/callback",
			Display:      "Github",
			TeamsToLogins: []types.TeamMapping{
				{
					Organization: "gravitational",
					Team:         "admins",
					Logins:       []string{"admin", "dev"},
					KubeGroups:   []string{"system:masters", "kube-devs"},
					KubeUsers:    []string{"alice@example.com"},
				},
				{
					Organization: "gravitational",
					Team:         "devs",
					Logins:       []string{"dev", "test"},
					KubeGroups:   []string{"kube-devs"},
				},
			},
		})
		require.NoError(t, err)

		err = as.CreateGithubConnector(connector)
		require.NoError(t, err)

		err = migrateOSS(ctx, as)
		require.NoError(t, err)

		out, err := as.GetGithubConnector(ctx, connector.GetName(), false)
		require.NoError(t, err)
		require.Equal(t, types.True, out.GetMetadata().Labels[teleport.OSSMigratedV6])

		// Teams to logins mapping were converted to roles
		mappings := out.GetTeamsToLogins()
		require.Len(t, mappings, 2)
		require.Len(t, mappings[0].Logins, 1)

		r, err := as.GetRole(ctx, mappings[0].Logins[0])
		require.NoError(t, err)
		require.Equal(t, connector.GetTeamsToLogins()[0].Logins, r.GetLogins(types.Allow))
		require.Equal(t, connector.GetTeamsToLogins()[0].KubeGroups, r.GetKubeGroups(types.Allow))
		require.Equal(t, connector.GetTeamsToLogins()[0].KubeUsers, r.GetKubeUsers(types.Allow))
		require.Len(t, mappings[0].KubeGroups, 0)
		require.Len(t, mappings[0].KubeUsers, 0)

		require.Len(t, mappings[1].Logins, 1)
		r2, err := as.GetRole(ctx, mappings[1].Logins[0])
		require.NoError(t, err)
		require.Equal(t, connector.GetTeamsToLogins()[1].Logins, r2.GetLogins(types.Allow))
		require.Equal(t, connector.GetTeamsToLogins()[1].KubeGroups, r2.GetKubeGroups(types.Allow))
		require.Len(t, mappings[1].KubeGroups, 0)
		require.Len(t, mappings[1].KubeUsers, 0)

		// Second run should not recreate the role or alter its mappings.
		err = migrateOSS(ctx, as)
		require.NoError(t, err)

		out, err = as.GetGithubConnector(ctx, connector.GetName(), false)
		require.NoError(t, err)
		require.Equal(t, mappings, out.GetTeamsToLogins())
	})
}

func TestMigrateClusterID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	as := newTestAuthServer(ctx, t)

	const legacyClusterID = "legacy-cluster-id"
	clusterConfig, err := types.NewClusterConfig(types.ClusterConfigSpecV3{
		ClusterID: legacyClusterID,
	})
	require.NoError(t, err)
	err = as.ClusterConfiguration.(*local.ClusterConfigurationService).ForceSetClusterConfig(clusterConfig)
	require.NoError(t, err)

	clusterName, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: "localhost",
	})
	require.NoError(t, err)
	require.Error(t, as.SetClusterName(clusterName))
	require.NoError(t, as.ClusterConfiguration.(*local.ClusterConfigurationService).ForceSetClusterName(clusterName))

	clusterName, err = as.GetClusterName()
	require.NoError(t, err)
	require.Empty(t, clusterName.GetClusterID())

	require.NoError(t, migrateClusterID(ctx, as))

	clusterName, err = as.GetClusterName()
	require.NoError(t, err)
	require.Equal(t, legacyClusterID, clusterName.GetClusterID())
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
		ClusterConfig:           types.DefaultClusterConfig(),
		ClusterNetworkingConfig: types.DefaultClusterNetworkingConfig(),
		SessionRecordingConfig:  types.DefaultSessionRecordingConfig(),
		ClusterName:             clusterName,
		StaticTokens:            types.DefaultStaticTokens(),
		AuthPreference:          types.DefaultAuthPreference(),
		SkipPeriodicOperations:  true,
	}
}

func newUserWithAuth(t *testing.T, name string, auth *types.LocalAuthSecrets) types.User {
	u, err := types.NewUser(name)
	require.NoError(t, err)
	u.SetLocalAuth(auth)
	return u
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
		},
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
