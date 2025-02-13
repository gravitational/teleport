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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
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

	cert, err := a.GenerateHostCert(services.HostCertParams{
		CASigner:      caSigner,
		PublicHostKey: pub,
		HostID:        "id1",
		NodeName:      "node-name",
		ClusterName:   "example.com",
		Role:          types.RoleNode,
		TTL:           0,
	})
	require.NoError(t, err)

	id, err := state.ReadSSHIdentityFromKeyPair(priv, cert)
	require.NoError(t, err)
	require.Equal(t, id.ClusterName, "example.com")
	require.Equal(t, id.ID, state.IdentityID{HostUUID: "id1.example.com", Role: types.RoleNode})
	require.Equal(t, id.CertBytes, cert)
	require.Equal(t, id.KeyBytes, priv)

	// test TTL by converting the generated cert to text -> back and making sure ExpireAfter is valid
	ttl := 10 * time.Second
	expiryDate := clock.Now().Add(ttl)
	bytes, err := a.GenerateHostCert(services.HostCertParams{
		CASigner:      caSigner,
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
	priv, pub, err := a.GenerateKeyPair()
	require.NoError(t, err)
	caSigner, err := ssh.ParsePrivateKey(priv)
	require.NoError(t, err)

	// bad cert type
	_, err = state.ReadSSHIdentityFromKeyPair(priv, pub)
	require.IsType(t, trace.BadParameter(""), err)

	// missing authority domain
	cert, err := a.GenerateHostCert(services.HostCertParams{
		CASigner:      caSigner,
		PublicHostKey: pub,
		HostID:        "id2",
		NodeName:      "",
		ClusterName:   "",
		Role:          types.RoleNode,
		TTL:           0,
	})
	require.NoError(t, err)

	_, err = state.ReadSSHIdentityFromKeyPair(priv, cert)
	require.IsType(t, trace.BadParameter(""), err)

	// missing host uuid
	cert, err = a.GenerateHostCert(services.HostCertParams{
		CASigner:      caSigner,
		PublicHostKey: pub,
		HostID:        "example.com",
		NodeName:      "",
		ClusterName:   "",
		Role:          types.RoleNode,
		TTL:           0,
	})
	require.NoError(t, err)

	_, err = state.ReadSSHIdentityFromKeyPair(priv, cert)
	require.IsType(t, trace.BadParameter(""), err)

	// unrecognized role
	cert, err = a.GenerateHostCert(services.HostCertParams{
		CASigner:      caSigner,
		PublicHostKey: pub,
		HostID:        "example.com",
		NodeName:      "",
		ClusterName:   "id1",
		Role:          "bad role",
		TTL:           0,
	})
	require.NoError(t, err)

	_, err = state.ReadSSHIdentityFromKeyPair(priv, cert)
	require.IsType(t, trace.BadParameter(""), err)
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
				Type: constants.OIDC,
			})
			require.NoError(t, err)
			conf.AuthPreference = fromConfigFile
			return conf.AuthPreference
		},
		withAnotherConfigFile: func(t *testing.T, conf *InitConfig) types.ResourceWithOrigin {
			conf.AuthPreference = newWebauthnAuthPreferenceConfigFromFile(t)
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
	authServer, err := Init(context.Background(), conf)
	require.NoError(t, err)
	defer authServer.Close()

	cc, err := authServer.GetClusterName()
	require.NoError(t, err)
	clusterID := cc.GetClusterID()
	require.NotEqual(t, clusterID, "")

	// do it again and make sure cluster ID hasn't changed
	authServer, err = Init(context.Background(), conf)
	require.NoError(t, err)
	defer authServer.Close()

	cc, err = authServer.GetClusterName()
	require.NoError(t, err)
	require.Equal(t, cc.GetClusterID(), clusterID)
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

// TestPresets tests behavior of presets
func TestPresets(t *testing.T) {
	ctx := context.Background()

	presetRoleNames := []string{
		teleport.PresetEditorRoleName,
		teleport.PresetAccessRoleName,
		teleport.PresetAuditorRoleName,
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
		err := as.CreateRole(ctx, access)
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
		err := as.CreateRole(ctx, editorRole)
		require.NoError(t, err)

		// Set up an old Access Role.
		// Remove the new DatabaseServiceLabels default
		accessRole := services.NewPresetAccessRole()
		accessRole.SetDatabaseServiceLabels(types.Allow, types.Labels{})
		err = as.CreateRole(ctx, accessRole)
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
		outdateAllowRules := []types.Rule{}
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

		err := as.CreateRole(ctx, editorRole)
		require.NoError(t, err)

		// Set up a changed Access Role
		accessRole := services.NewPresetAccessRole()
		// Remove a default allow label as well.
		accessRole.SetDatabaseServiceLabels(types.Allow, types.Labels{})
		// Explicitly deny DatabaseServiceLabels
		accessRole.SetDatabaseServiceLabels(types.Deny, types.Labels{types.Wildcard: []string{types.Wildcard}})

		err = as.CreateRole(ctx, accessRole)
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
				require.False(t, types.IsSystemResource(r))
				createdPresets[r.GetName()] = r
			}).
			Return(nil)

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
			Return(nil)

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
			Return(trace.AlreadyExists("dupe"))

		// EXPECT that any (and ONLY) expected system roles will be
		// automatically upserted
		roleManager.
			On("UpsertRole", mock.Anything, mock.Anything).
			Run(requireSystemResource(t, 1)).
			Maybe().
			Return(nil)

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
			Return(trace.AlreadyExists("dupe"))

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
			Return(func(_ context.Context, r types.Role) error {
				if types.IsSystemResource(r) {
					require.Contains(t, expectedSystemRoles, r.GetName())
					return nil
				}
				require.Equal(t, teleport.PresetEditorRoleName, r.GetName())
				return nil
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
		}, presetRoleNames...)

		enterpriseSystemRoleNames := []string{
			teleport.SystemAutomaticAccessApprovalRoleName,
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
				_, err := as.GetUser(user.GetName(), false)
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
			auth.On("CreateUser", ctx, mock.Anything).
				Run(requireSystemResource(t, 1)).
				Maybe().
				Return(nil)

			// All attempts to upsert should succeed, and record the being upserted
			upsertedUsers := []string{}
			auth.On("UpsertUser", mock.Anything).
				Run(func(args mock.Arguments) {
					u := args.Get(0).(types.User)
					upsertedUsers = append(upsertedUsers, u.GetName())
				}).
				Return(nil)

			// WHEN I attempt to create the preset users...
			err := createPresetUsers(ctx, auth)

			// EXPECT that the process succeeds and the system user was upserted
			require.NoError(t, err)
			auth.AssertExpectations(t)
			require.Contains(t, upsertedUsers, sysUser.Metadata.Name)
		})
	})
}

type mockUserManager struct {
	mock.Mock
}

func newMockUserManager(t *testing.T) *mockUserManager {
	m := &mockUserManager{}
	m.Test(t)
	return m
}

func (m *mockUserManager) CreateUser(ctx context.Context, user types.User) error {
	type delegateFn = func(types.User) error
	args := m.Called(ctx, user)
	if delegate, ok := args.Get(0).(delegateFn); ok {
		return delegate(user)
	}
	return args.Error(0)
}

func (m *mockUserManager) GetUser(username string, withSecrets bool) (types.User, error) {
	type delegateFn = func(username string, withSecrets bool) (types.User, error)
	args := m.Called(username, withSecrets)
	if delegate, ok := args.Get(0).(delegateFn); ok {
		return delegate(username, withSecrets)
	}
	return args.Get(0).(types.User), args.Error(1)
}

func (m *mockUserManager) UpsertUser(user types.User) error {
	type delegateFn = func(types.User) error
	args := m.Called(user)
	if delegate, ok := args.Get(0).(delegateFn); ok {
		return delegate(user)
	}
	return args.Error(0)
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
func (m *mockRoleManager) CreateRole(ctx context.Context, role types.Role) error {
	type delegateFn = func(context.Context, types.Role) error
	args := m.Called(ctx, role)
	if delegate, ok := args[0].(delegateFn); ok {
		return delegate(ctx, role)
	}
	return args.Error(0)
}

func (m *mockRoleManager) GetRole(ctx context.Context, name string) (types.Role, error) {
	type delegateFn = func(context.Context, string) (types.Role, error)
	args := m.Called(ctx, name)
	if delegate, ok := args[0].(delegateFn); ok {
		return delegate(ctx, name)
	}
	return args[0].(types.Role), args.Error(1)
}

func (m *mockRoleManager) UpsertRole(ctx context.Context, role types.Role) error {
	type delegateFn = func(context.Context, types.Role) error
	args := m.Called(ctx, role)
	if delegate, ok := args[0].(delegateFn); ok {
		return delegate(ctx, role)
	}
	return args.Error(0)
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
		KeyStoreConfig: keystore.Config{
			Software: keystore.SoftwareConfig{
				RSAKeyPairSource: testauthority.New().GenerateKeyPair,
			},
		},
		Tracer: tracing.NoopTracer(teleport.ComponentAuth),
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
  id: 1736815462679560000
  name: me.localhost
  revision: 1a206afd-01ab-4fb6-9bd4-8615e17d60e0
spec:
  active_keys:
    ssh:
    - private_key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcFFJQkFBS0NBUUVBdlpkajF2dXdoUktJL3BFVjkxdllEVHJUOHBpVzVrQjBKVXU3UGxUa0tiVG9TbVRsClozQ1lSNk5xMkVNYUE2QTIrZDJUT0Vyb2R1Mlp2K2tjWlYxK055T2czKzhQcUtiWG1JampBV3YyYlVFZEtmZUMKTW9Rb3Q5Rk8yelI1SkNEVVBWOWduSS9PbHU4WUdYdWJqYnZQU3k2NFM5RmQ4ZjM5YzhMdGpYbkUzZXRLTUtRcgpUVkpFb0dxa05YVmJUd2x2RkFUbUdUcmZHSDJNdG5FdTZMMEozLzNmT1pmOG1SRXBWMXg4YkZjNUEydWxmUDZRCjFqOUJaOFBTRmFlV0pEWFV4dzJwSzl4c0pPaW9LNEREcHU1K0ZZcTRqQnpmS1ZyamF0ejJhVjBKWVRvWFpiY2oKK2wwVXpKemxQRUdjSjBzRmpkU3grV0RZdS9sbTljT29hdzEzSXdJREFRQUJBb0lCQVFDODB5TGs0eGdUOFRudwpFS0JJRkhsQjgrMVVHUlZ4alpBZjlTVXdGMnlHL1Y2OWVXL2hiZ3E4anMzRFJsR0tldTlHUEtCNzJGOWUwNVhsCnhVNDZ4cnNHUDczaVNqN1dRaFZJSGsyNUJNWVNXbCtwaEpGdnJxQy9Ndi9PNHB3a2wyM0xFa3N1b3l1bXQ4clEKMW9NK3ptYlBBbUViWWhLbkNjaDhtdy90Yi9IYTh0ckpyWWVDaGlMV0cvN0NSZU9uQlZubXlnN21tTnNYNHFTQgpFcFlYSUdIOXhRN0VoUXJ6UGxvTWRJQWtLTGdiZE5MM2tkdzFMRFBhaGhwanhEK09VTEJVYkx0a1JrTEM5ajYvCml2dnUxVXUyZjhhYklseTNaNWxQTUlINk12Unh4bS83d21FRVowY2JjM0dSb29ZSTJPOFdjamRYTjJpRFBWQmYKNDFmOGNvMUJBb0dCQU1RZTJQSTF5NE0rZnVpUll1N1dvbWNETVdGUm1CTGFKOWo1V3NYWXJLamtSS1owMk8wRAo5dHlZanpHT2VYZ2pSOHFGNHRJL0RaTHdZSWVPREtpWFN6RlplanpGdG9HK3pXNWFDM2MxOGswem5VeDhpQnpBClhic0wzRmN6ckw1OUVtbDVwVVNmRUhqWmJPUTU5bG9DaEhqSDl4cHVoVUo3YnBidFpPd1d0REFsQW9HQkFQZDYKTnBhZjdjWkN0TFlJQk5pSzhTV1RIZXZSUFBHRUI5Y055b3VxQ0svdS9Rb3JITkRNRWE2UTMyRHI4MnJ6V3Iwago5U3NIL1plNU5UWVlxZUdtRkZmS2RpWms4d2JpUWtCYWFNd1N5ZEprc01ocjZ3T0NXd0J2QjFUMmFodmU3Mm9qCitTa1lqMzFvOXVrY2trT3pBa0pwelZrQTdwb2g1V0hSZ25GSEtTT25Bb0dBSndhQVl3b2pXaFZvaVh6TXMvd1AKeXZIT3RLL1kwLytIS0Z6T0hFcDJhUkVyTy9oS1pqZUF1dnE4bTc3Zkd2SGlTa0dFRmhRbjdsSlkwd0NJTWxBUQp6VndodjlBVDloTnlxMy9OZ2taQTFlM3NZaGp4dU03cWw5clBXS2JXdS8wRldlbXo0a2pJclZPT29JZU1Kdk1UClN6bDNTVkl1d0VEeGk2VG5qVGNqV2VVQ2dZRUE3RWYzVHFDcmVKdS94ZnlxQThYRXI4ZGl6Z0FjVzh0ZllPaDkKOWhNRjhGUVJyRisxUjNWUGZJZzlmbUJKTEZma3pxbENMeStWNUFLazExMTg5VUNJTTduT1RLSWRsdmozb0ZHeAp0UVpMUTJGM21DUFJZcXhYRG5ielhSOVg5L3hHUWVUT3czbjdwaFZOaVF3S2FqRERlMzFnM2hXUnVmK2E3bVlHClVQbE1RZ2tDZ1lFQWx2ZVR5THVGK3JlQ0xZUjgyRllBYzZHV3o0bFNzMWE1RzB3NFBnQXI0ZFVuU1ZSRVpaeHAKTlZabVZvdW5weFJWdHNCVjV4dVp5YXpzQ2hCNVBsZE56d3ZRS0JXaTlvQWNyOEpKVkhvKzJHM3IwYURUK0xlSQpFUFVqRkJvMFZrZ0VvU05tazBBOXY2N0lBUzZvTk00UkY4cHJvRm1CWEliQTdXZ2V3c25kWHM0PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
      public_key: c3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCQVFDOWwyUFcrN0NGRW9qK2tSWDNXOWdOT3RQeW1KYm1RSFFsUzdzK1ZPUXB0T2hLWk9WbmNKaEhvMnJZUXhvRG9EYjUzWk00U3VoMjdabS82UnhsWFg0M0k2RGY3dytvcHRlWWlPTUJhL1p0UVIwcDk0SXloQ2kzMFU3Yk5Ia2tJTlE5WDJDY2o4Nlc3eGdaZTV1TnU4OUxMcmhMMFYzeC9mMXp3dTJOZWNUZDYwb3dwQ3ROVWtTZ2FxUTFkVnRQQ1c4VUJPWVpPdDhZZll5MmNTN292UW5mL2Q4NWwveVpFU2xYWEh4c1Z6a0RhNlY4L3BEV1AwRm53OUlWcDVZa05kVEhEYWtyM0d3azZLZ3JnTU9tN240VmlyaU1ITjhwV3VOcTNQWnBYUWxoT2hkbHR5UDZYUlRNbk9VOFFad25Td1dOMUxINVlOaTcrV2IxdzZockRYY2oK
    tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURpakNDQW5LZ0F3SUJBZ0lRSXoxQlR1aHZqUFBvWVk3M29UNUZTREFOQmdrcWhraUc5dzBCQVFzRkFEQmYKTVJVd0V3WURWUVFLRXd4dFpTNXNiMk5oYkdodmMzUXhGVEFUQmdOVkJBTVRERzFsTG14dlkyRnNhRzl6ZERFdgpNQzBHQTFVRUJSTW1ORFk0TkRFd016UTFOamt4T1RnNU56SXhPRE16TVRBd01EazNPREUzTURJeU5EYzNOVEl3CkhoY05NalV3TVRFME1EQTBOREl5V2hjTk16VXdNVEV5TURBME5ESXlXakJmTVJVd0V3WURWUVFLRXd4dFpTNXMKYjJOaGJHaHZjM1F4RlRBVEJnTlZCQU1UREcxbExteHZZMkZzYUc5emRERXZNQzBHQTFVRUJSTW1ORFk0TkRFdwpNelExTmpreE9UZzVOekl4T0RNek1UQXdNRGszT0RFM01ESXlORGMzTlRJd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDanVtZ3c3bDlkNUNMMGpxdHlPbCs5Y1hrMXZkSDhYL1R5RmhjQ0lWZnkKaDBJOFdlS2lWMGFzQnFIYnVJOWpDcnF5ck9wS0NieFlxV1k1QmpMb0YzNUVjblc2U3dLUEhGU0hwRWoyMEZLNwordGdyckxOdHVSM2Y2WUp6ZytML3M4SkF5Y0U3TTRIZDNQbnhYM3hiOE1mUlBRNHh3L2E1SE9tZ0dFcGhKZG82CnhqQm9KQ2dkNUVsL0MxSnBPVE5Fd1MraFpJSUxpcVZKUHV4T20xWkUzYnJUY3VYL0tHR0NndlVGSlpxV0g4ekwKQ011QVVkWG5WUTBxVUZXOUxGUnM1Z0Nkb1BjVzl2TGxLS1hyUW0yZXpHbms1NEJQbHR5L1V5RTNXNEo5WCtOTApQaXlEeEtZOTQ4eTlVazVIV2FlYWZsRWRQbkR6b3VKWmg0Nk9EelpmOWthdEFnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQnBqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCUm94bWZGSWNPSCsvckUKUzlQM1NncCsxenAvdnpBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQUNTUEVRMU5UazZPWGp5ZHhYbEpqUThrYQpqbUlFcG44WUQzU1NHV3duSkRqbUVDbXU1NXFZYmFhY3pBM3NackZ6Q2lLMWpyY2lBNUEwVGhkNFkxa1N5eXJTCjlteGswZTNHQ3IzUVRVeWF4T2FjZVVWb1hhK2FGMmJrZi96SnN1ckdOL3ZHY1duUkpBWmZxUUdmVWphN2MyLzkKbEtGbE5PazkrbTZUd2VCbGZvUWYvR01OaTBDSTNpTDQ0RFBsMzVaN0hFSlFqYkdGNWMyYkhNM3I4TkhMTm5QYwp1YUJKNFZSRG02N3N6aHI0UERvMm9YSkFGVDFSSGxnd0UvbWZQQTlscklaRFBXRGlBckhoMFpqSTR3T2xDTDBpClFlRDBraDlkeXB6b3FwdS9LQXlTMm1wZ1lVU3huazRJTEhPNDdnZGc0MUVPTGk1ZE1PcDRNOTRFMTVLdTZnPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
      key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBbzdwb01PNWZYZVFpOUk2cmNqcGZ2WEY1TmIzUi9GLzA4aFlYQWlGWDhvZENQRm5pCm9sZEdyQWFoMjdpUFl3cTZzcXpxU2dtOFdLbG1PUVl5NkJkK1JISjF1a3NDanh4VWg2Ukk5dEJTdS9yWUs2eXoKYmJrZDMrbUNjNFBpLzdQQ1FNbkJPek9CM2R6NThWOThXL0RIMFQwT01jUDJ1Unpwb0JoS1lTWGFPc1l3YUNRbwpIZVJKZnd0U2FUa3pSTUV2b1dTQ0M0cWxTVDdzVHB0V1JOMjYwM0xsL3loaGdvTDFCU1dhbGgvTXl3akxnRkhWCjUxVU5LbEJWdlN4VWJPWUFuYUQzRnZieTVTaWw2MEp0bnN4cDVPZUFUNWJjdjFNaE4xdUNmVi9qU3o0c2c4U20KUGVQTXZWSk9SMW1ubW41UkhUNXc4NkxpV1llT2pnODJYL1pHclFJREFRQUJBb0lCQUZEaUxjYStlKzV1WGJaagpKTjl4WndxM25DR29mS3dvMjJFYytKRGMyQTNBTkVDTVJ5SGI2OVhnRU9YeTd5TUdrZVRpOTN0TUEvZm85ODhECitQSWZhUWwzWWlGK0hPMkdHVnhKRktLWmw4VzF6a1VGTkQ3b1RKSHBVY0N2VHR6emVPdDR3RFQyNVJrdHFXeE0KdDZyVDhHSzF2dVZtNGVQaEhLa3lWc3hYWHMvWmZvTHhxa1BxWXFHRkd4NU1VelVJY0hnQmpyRVVIWXJVNHN1VAoycklzOFl4Vm5nYWdGdXdmSk1WdkN1aDVvNndyWHJvbHJKdDlFY1BtMzFVWFpGTXlhaTRVa3JVbk5qbkNMU3JsCmYxalEyQWRkYWp2UE5mYTk0ZFBsRVdOeHlRM29YNEVwOFRqUFhTc3NUdURXVFdZVUFURXFIb25MNEZUYytNVzkKTXh1SjJ5RUNnWUVBeHVjRDRwaW1TaDBkTGZPZTBVM0VmQ04zNll3UDVXdDVIMHJJTzBQQ2l5ZVNoSEZvaUVJMQp0eTZnWTk2UFFEYU1ycW5OZDN1Y3Q5WjYwVHlwUloxbUtNa01hV2hOVG41L1NqbWNxTTV4ZVZYSWVvV1QrRWkzCnQ4bmd3U1ZlZUQ1OExnQ3dFaFF6ZUJLM2l6UVVQN3ZPclRXcWlRNmxUZGNXa1JPRWhQL0lDdE1DZ1lFQTBycC8Kbnh4enh2MXEyUmY4ekZvVTRvMG5Mb1dvRFYwM1BTcGIwU0xUSzYwVndFZGNESVRqSnlmbTBQRFcyNXJobmVhOQpQeGw1R1NtakExMFpKNlprZVVIMkJwMlJjNDhraHd4Rit2bVkzSUZKeFlLMXBGWlEwOS9yYjkxendESWhtSGQrCk10b3JvbE5jb0srcC8zWXN4SkZFOXJxVzVjbmhLYnlwUFRzQ2VIOENnWUVBa2tuTWNMZEc3cEdWS1h2Wm5pVXQKVXdRZktKVk1CN2RRNFRQMktxaCtpQ3cxdGRWWFJZZzB5Nkt1Y21WNVJJZ2FWa2dyQnlyU0s5L0NldXU3cjZqQgpQMVFISGV1Sm1DYXZaaDhUV3BCam94TDFuUzlya2h1aGk3b2Q1TkNnTjUzMVpUdzZRMEc2VFNDdS8rSHcxcU5CCnNlRWJxU3d0WmgvQXlEanJxWW9hVGVNQ2dZQUZRZGZiUFZkNkdHcG8vaHMxY2UzaGRRb01OQk5zT2U0ZDNZZXEKNFFhSnFXaklnajgrcExZU0RRSEtKcWdGbElpYWF0NC95Ny9rcTlCQVRqdEpiUEpHd0NtR0lybzFPdFg3ZElmdQphZm14VHB4cmpBWkNFbEV6NS9zMHNENnFCZFltdXB4d1lsY0NWcmdSM2pBTWlvTTFhRFpqUFdaMFZ5UUI2WTREClZBeU11d0tCZ1FDdlA0RUVHOUcxSFYzQUpXVkVhSmpLai9paEhJTFdYTk1ZWVR4OVVHdjd1My93d1hiQ0tTRDgKTG4wdkp0UVFQNnpISGtSV05GM003U2RlU0VGSHQ4SWlXTThueXFTOUxOVkJtSW8vNVNUUWwyTlZYdmdYVlZNUwpLUU1lVlZHTkR0Ykd0REFhclo1VFZhTHFqMmNGaWF1U3h4SGdneVU0NUlrVzlHQkJqM3g3WGc9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
  additional_trusted_keys: {}
  cluster_name: me.localhost
  type: host
sub_kind: host
version: v2`
	userCAYAML = `kind: cert_authority
metadata:
  id: 1736815462679182000
  name: me.localhost
  revision: ea729b7f-99e0-4330-a46d-71c8dda9d3f7
spec:
  active_keys:
    ssh:
    - private_key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcFFJQkFBS0NBUUVBbnBZd1JWUXhZS0VTWVJJZVdIQXhGRGtFNmRwM2R1MEFyUlNrSnlhRmlzVG9wRzlGCkV6SDRuRkpPU3prbW5rWVE2R2hJZ0ZxdFRURDhtVGt3bVJGZy9LQXViVTBrSmxNbldkQW54dHAreVkvRGNPUTIKV2dGN0o2MlVzSG85U2VDWnVCZG9WM3ZpaUw3cm9XVE1hTUYrZmhTMlN1bVZaUEl4U1l1aEtNcjlZK2dVVzhULwp2NUI4azA1MTVyTGxrVVU1aC81dm5YVEVWMVRmSTdFTmc0V3FZWVdOc2JDWkpjUEZ5RFMwU0pVdUtwbEt6RW9SCk80aDVTZzRkTnZDMFFoc1VvL0pUVmE1ZXQyUi9WQ0NXN1FjNHdhbklaQ2tGNXoxbHczcU9CQ0VSdlJRUW5zVk0KZm90eVh4VGY1M1hGdEk4ZE5yekJPeWVXeVlHNGFhR2doYytweVFJREFRQUJBb0lCQVFDQUVXK24vVWJtN3d6RgpvWGtxR0dnNkdaWHpPSDh6WmxBZWRrWGViQWg2T1d4YXBwVVUzRTBXQ0kyN3g4cDlGTDVBd1Q2VGtTYlU2Sk9GCk5aOGViZDl5QS9XYVJTckZYRyt4NHh6TVJOVVE5MjF3dEl1RUFpQXZ1Y2tTLzVTUkhiVmw2bGxVRlBLclZlczUKNmduOUt3MTR5a2N3bGhRVWNsWUZPNktKSyt5WGlhV3lpSXZLOU1RWlBEUUtEWG5sUitPeGw4VWsyTVk4NmdPYQo1a2l1M3lZbFZla0NiMjVETG5renhsRFVuQ0QvT2pSaW1uVDB2MjJnRzZ4Y3ZRdG8rSlBtYmxwaEZ0bk1hQTI2CkFSOW05YmVvWDVFRlArdDRNNzhEVEFRUFkvcW0rN29lTlMrRkRmL1N5YlJsWlBFaE43dTdTN3dZWmVUdGs4cGkKb2ZSWDRrd0JBb0dCQU1YcXZHbmQzZ3Y1WGljRjh3SU5ISnBzd04rS01YVWN1NHZZN0NlOWVycllvb3djNnI5TApadnVRbWN0amdmdG5HWDhrSVRmNXdsenRjQ1gvNVJmam5OeE50bGJubE5qM0pQNDYxZWVHMmRKYTJzWWd0VE5DCnBpNE1QZTJ4M1QyZFV1N21UMEcyUnJJMW9Kc294b2JCMEQyYkJnVlIvV0thR240OWJpLzd3eXVCQW9HQkFNMGcKbmQ2SURMR2NiYVlhV0xJYUwyWDRFckJJTmFGRU9URVBrSHNMTUYxVUJJZXRNcDl4WGNLaTZZWHJlRUZva2xqeApWY0RDM2hzQ2VQK3hQY29icDZ5UG9Sd29yREpGcnVWTU04Wm1HWEI5ditWakpSeHpYUGdUSURUN0U4WEEzb3pSClQvT28vV0s0blFGUTdqMnFaMUdOMm51TzZtQ3JTUzRGSGJZMmNVSkpBb0dCQUt5QVVPSXhCOVVGN3lNeUUwRUoKYnBIR0VrR0Q4R0Z6dnA5QVhXeXh3S1BVSjdEWmoxMVYraGR2VEN5eXVWc0czSGt0WTJxblhObWo5YWlaSmZNeApac201VGlEbXpaeGhwTE9WVWxUdSt6RldFUEs1RlZYdFZHdzBMVkhjUWNudk1wYVkxQ0doSG5NN1BKV2Y3NUVLCm9sYmZwRnJFd0lYTmJTUDBwUEpiakJ1QkFvR0FReXN2QnJOZUZMcTRYTys3bzNaWGx2aElobGplMXRQVU5uQjIKU3hRNjNoU285eFNMd3hJSU5iZks2QU5XK1hRWWwrOU91VFFXTHBuOHJSMklzaW1rR2lsZUJDNTlWR2psQUVpWAptNXZMTUw2OG00eC9sblZnT0F0clBHNEs1M0prYlpBTXNpamY3L2VyMGNhQ2ZNYlQxaXl4SWt5R0N1bUxxUG9iCjVKS25PNkVDZ1lFQW9JTi9vd3ZqbVNIczRRV1hGMEpacHhsYmp4QW1FdXdkNXBBOEpiUitlc2VwZE9MYVNRK2YKM2lyL2lIZXgxem5mcHhyMUdqRS9rQmNCM0lrRTA0emhwRk5xTFUvYTI2bTBZMWI1RkxmVEtMNHpvTklXU3BuUApFKzVmQnkrSkhVczc2dkJWM2ZaY09WRjZqc0RpMjUyV0NoK2VrbFNVVWpLQmdpa1dNVVpDMnA0PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
      public_key: c3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCQVFDZWxqQkZWREZnb1JKaEVoNVljREVVT1FUcDJuZDI3UUN0RktRbkpvV0t4T2lrYjBVVE1maWNVazVMT1NhZVJoRG9hRWlBV3ExTk1QeVpPVENaRVdEOG9DNXRUU1FtVXlkWjBDZkcybjdKajhOdzVEWmFBWHNuclpTd2VqMUo0Sm00RjJoWGUrS0l2dXVoWk14b3dYNStGTFpLNlpWazhqRkppNkVveXYxajZCUmJ4UCsva0h5VFRuWG1zdVdSUlRtSC9tK2RkTVJYVk44anNRMkRoYXBoaFkyeHNKa2x3OFhJTkxSSWxTNHFtVXJNU2hFN2lIbEtEaDAyOExSQ0d4U2o4bE5Wcmw2M1pIOVVJSmJ0QnpqQnFjaGtLUVhuUFdYRGVvNEVJUkc5RkJDZXhVeCtpM0pmRk4vbmRjVzBqeDAydk1FN0o1YkpnYmhwb2FDRno2bkoK
    tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURqVENDQW5XZ0F3SUJBZ0lSQU1GY3prVVhnOFV4WnRBMkI1dStUUFV3RFFZSktvWklodmNOQVFFTEJRQXcKWURFVk1CTUdBMVVFQ2hNTWJXVXViRzlqWVd4b2IzTjBNUlV3RXdZRFZRUURFd3h0WlM1c2IyTmhiR2h2YzNReApNREF1QmdOVkJBVVRKekkxTnpBeU1qZzNPREUwTnpnM01qazRPVEEyTmpNMk1qY3hOREV4TWpRMU16a3lNakF6Ck56QWVGdzB5TlRBeE1UUXdNRFEwTWpKYUZ3MHpOVEF4TVRJd01EUTBNakphTUdBeEZUQVRCZ05WQkFvVERHMWwKTG14dlkyRnNhRzl6ZERFVk1CTUdBMVVFQXhNTWJXVXViRzlqWVd4b2IzTjBNVEF3TGdZRFZRUUZFeWN5TlRjdwpNakk0TnpneE5EYzROekk1T0Rrd05qWXpOakkzTVRReE1USTBOVE01TWpJd016Y3dnZ0VpTUEwR0NTcUdTSWIzCkRRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRREdCK0c3UE1OR1FCYXUrQUJvL0xPNDdIMjQ5ZUpqeEhDRnlrWkEKeDMxYWphNWs3M1RLaWlFVmNBOXJxZ1FBWVJkcGZRdUU4cUR6SExPTGZFRTB5Nm9ERVhhNHJtV1hGam1wRjVlNApmTmYvdUZqMkJPMFI4N2JoQ2hOSHY3cUh5YU5vVjYxNXdKNStRMkVNblc4R2FoTEx5YnliZzRtZ2QwZEZkaHBrCnE2SXk5OFJTQlQ4WXk1aG5WT09YOTFJemFOWlpKQ2tHUWlkZ3d1R1lCRkdUcGNMQjhQVUVYOVZPNDVMYTFZYUkKTS96NDZ4TEdmcWVlWWdHZURObC96S3JDdVpZc0s2eHkrdERVek5ILzBWT3Jpb3c0aEhWbi9SdHV5eEJyZTRuNApwVHovVXRDWjVZQWVmTVBIWWo4UlBFWFlqTmg0MlY4bVpobkc4TWNVRFU2cWxUa3JBZ01CQUFHalFqQkFNQTRHCkExVWREd0VCL3dRRUF3SUJwakFQQmdOVkhSTUJBZjhFQlRBREFRSC9NQjBHQTFVZERnUVdCQlQ3RTZrUEh2dFQKanMrUlRWSVVLclhBaDlKTmNEQU5CZ2txaGtpRzl3MEJBUXNGQUFPQ0FRRUFRb01lRE1xVUt2VFE0OHEyemRpbApQc3p2dzFNdlhveUxHZWw3L01wclVEK0l6THl2cU1DUzZmbnhvV0NSRy9SbjAwaHorS1BIc3gva0RubHVjNVhKCnNqYjEzMjRhdDVhQlB0NmRibUpUR0lqK1NnenlUWmQxVVpSdTgxS0xCRWlxa0FMUE15aG1LYUEzSXhsYnk5S2kKNGpQNWwwK2JwanNTR3ZNTzNaK0thY1JGQlN0c21kbkJXZEdSMW5Hck9mVlBPRWUzSDg5NTFkbWpFNVdrTnNEUgpUNXNjVXRDYzlCdDduNVU0S0k3TE53SFA3cWppc0dpbXZuTWlOcVFOdjIxS2VEZ3E0eko3SkxrcENNRzlxRDB2CnFTQWlnYXhWL2o5bTVWcWF3d3k2VDlsUDh2WHhoZGV3dkhSY0hrSTdNc2VFYnYwdGlyelRLdDlzTVM1UGd3czgKQ0E9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
      key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBeGdmaHV6ekRSa0FXcnZnQWFQeXp1T3g5dVBYaVk4UndoY3BHUU1kOVdvMnVaTzkwCnlvb2hGWEFQYTZvRUFHRVhhWDBMaFBLZzh4eXppM3hCTk11cUF4RjJ1SzVsbHhZNXFSZVh1SHpYLzdoWTlnVHQKRWZPMjRRb1RSNys2aDhtamFGZXRlY0NlZmtOaERKMXZCbW9TeThtOG00T0pvSGRIUlhZYVpLdWlNdmZFVWdVLwpHTXVZWjFUamwvZFNNMmpXV1NRcEJrSW5ZTUxobUFSUms2WEN3ZkQxQkYvVlR1T1MydFdHaURQOCtPc1N4bjZuCm5tSUJuZ3paZjh5cXdybVdMQ3VzY3ZyUTFNelIvOUZUcTRxTU9JUjFaLzBiYnNzUWEzdUorS1U4LzFMUW1lV0EKSG56RHgySS9FVHhGMkl6WWVObGZKbVlaeHZESEZBMU9xcFU1S3dJREFRQUJBb0lCQVFDVUdFTGMxcFVtalRrcApnbmcwQzMrUU5QUFVoYlhYYkluRjFENXpwWHgrWXVSZndaL3k5QmZIdzNVVXpDR1A4d3dpTEl5WDBTZENpRjFSClhBd2Jvbyt6R2JWU2FjRzVtcnBtVlNsMm80NlpROURyczBWam5vSk9pMDFkNCtsb01RaE9PUHVYeU0vK2x2OFcKQXdxTG5ub09Bd0ZVdjZzRjRRM2d5WEQxaGxHWGtOZnlTWEI5QlkyQmZ0dGswREszMEVqeEd6NjJpVWRITEJObwpTUllEalpBRHZpeTZzamcxWVBFczc1UHc2Y0JSdzRteTF6MWRyRnlESHlMbVd1NWZlMzhMd3pWeXQ5SVc2N2tBCjgrRUVlZTU1c0RNdml5OUd1Zm1mMlNyV0J3L3BDSitNbkVTYkhNWDQ5U3U0Z01rTndPc3VlZjNrK1hHTmFrdU0KZFpsZko2MHhBb0dCQU8yMG9vcUxHS2hhUk15c2FEaWNESXNaWU5FTzRRc1FaME9iOVI5RTVqQmY4TmtnTkZIcApiWHF1aFJmSmNHN2pTZmN3NGRLOGx0aUVnek9sTVMzeUlJV2FSeEc3OXIxNTllaXB5ZDBtbTdTYXdtSkN6VmsvCmt1S1p3SE5wWnI3ckYvVWcrSkNQODMzSXBZS1YxWGNrZER5Q0RIY1J2aFVKT0VmZnlwWjR0bzNKQW9HQkFOVkYKandhOEdrcy8rUDRiRytXZExJOHZ6TFJxT3N4dEZ1bHNUNlBtcGxzYnF0OVZNK3hkd0RzMk5WeGNtK0g1TnhrZAo5bUVJMVljaDh2WFVyMTBiYllhTzNMU2lKYm1uK25vNDNOM3h2dEcvU2N4Umc5RjlQbm41NU54UzRrMGdKdWEwCjNUSmdidWt3UmtIbkpwZ2pUbERURjhrK0RmZytFRWtPSjk5ZTk3bFRBb0dBYkdpaWJLOE5XdEo0YUNRRkVEUlQKSUNrOTEzcUN0aW9QL215bE9WS1I3T1FFa3ZHMkN0bDd2YVRVUEVuNWhna1ExYlNzZVJEYmR2blFZSUJwVW53SAp5d2JXZk1jTnU5SmdqWERLQ0pzd0RnazZ0OWVoa1orRjNPU2tPYjZMUm0wdnF2TVRpZEt0Q09PMllEejNjdlBrCk15aFlpUUZGZ0pDSTQzYTBEVFlXZzhrQ2dZQlJvM25YaXlQSmtHaUE1T0d0NkplSkREUWhEOVVJTWU0bVZtYTYKQiszQVRId0JWNzB6aXNPdUp0Y1FUd2NBM29RLzRoOVJENitsTmRLcVZjcjNLaXVuNllJRXgxa0hrNHluUXFNUgpkcHVqOE1TUUtOZjcxaVNYVHBoVDJvcDBHWTJxbkt0YndGeFVlVDA3dHY4b0Y4Ty8zcjVwTTQ3bmF1S1RCSTh3CnkwcXFyd0tCZ0NHT251RjFGUU5LdW85b0g0YmhOVW1xbVhmYng5czMwQm1ZTmVHUWczZ1F5amVzYmh5MDdVV0YKWnkzQXZNYmdlL3pralpaSnZPcFN5QzlLWER2aVBGK2UwRG9WY0Z5VFI3TVlyLy9IcVBlVHcwVVVXZnlQb2FxWgp5QXI1cUU0MVR6NGsyR2FOYnN4R3hpU2J2cFdWa2gxMFdjdmpwdHRka1I3UW9RSlg2UU9xCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
  additional_trusted_keys: {}
  cluster_name: me.localhost
  type: user
sub_kind: user
version: v2`
	databaseClientCAYAML = `kind: cert_authority
metadata:
  id: 1736815462678741000
  name: me.localhost
  revision: 2ab94406-c75b-4894-bc63-04191d54c15a
spec:
  active_keys:
    tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURpakNDQW5LZ0F3SUJBZ0lRTjlDcHA4VkYzQUpuWGFnbk5kVHVkekFOQmdrcWhraUc5dzBCQVFzRkFEQmYKTVJVd0V3WURWUVFLRXd4dFpTNXNiMk5oYkdodmMzUXhGVEFUQmdOVkJBTVRERzFsTG14dlkyRnNhRzl6ZERFdgpNQzBHQTFVRUJSTW1OelF4T1RBNU56ZzFNelF4TWpRM056VTVOVEl5TVRZeE5UZzBNalUxTURjME9URTBORGN3CkhoY05NalV3TVRFME1EQTBOREl5V2hjTk16VXdNVEV5TURBME5ESXlXakJmTVJVd0V3WURWUVFLRXd4dFpTNXMKYjJOaGJHaHZjM1F4RlRBVEJnTlZCQU1UREcxbExteHZZMkZzYUc5emRERXZNQzBHQTFVRUJSTW1OelF4T1RBNQpOemcxTXpReE1qUTNOelU1TlRJeU1UWXhOVGcwTWpVMU1EYzBPVEUwTkRjd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFEQzMyUDdINUs2T2tYZE1Hb3NsWjhMMDR2eWRMSGxYRGhEcHEvMnVTYW8KcVA2MjMyaTd3MXE1VThPblVPWFA0Nm5OcWFpclFMUkpHczZuaGsxdGNwMEZkQVlvRGhGdkVuVnc1WmVxVzFZSQo3YlpGKzQvM00xTnk2VmU1cy84aTJXRkdkd04xaHhKTE9VcENIRm5jTmlkYkFzL3hFSzdVaWdWVnprMmx6bm5QCkIzeVU4Y1hneCtuZ3lIbncwcFRwcjdIdFJjdW9TbkROSkthN1I3YkwwenN3aVB6V1RXbGhYaE44LzJCMUJ4MnEKSkZVRVltS1Znc2lXK2ErUTBJbW1PVm9JNXF2QjRRdE11NHVzWWc0enovS3NhMExWQnVwUWVwL2lleXlka211cgp6K0p6ckIxU3BtTDRGY21YTnIvUHFBbWtENmNkQ2lWVHNkNDNBdGxSak9NUEFnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQnBqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCUVp6cGh0aHVQdjlLdUMKd0d6c1ZrKzU0NkRMN2pBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQWlOM2lMeWdyZHVVU3MzYS90cS9NKzJmbwo4K0EyaWZYM2dOc0Yxa2l6NUhnczhvbnNOWmhSSDgrNU1ZSXVoSjNDYllGSHdCM29QNmdPSUdVWVlwbnExTTViCnR6WThycXZMaWpEZEpHUHR5YTRsVFhmY25saGljRE0yeE85WUpVT2xEVjZZdWNRV0VpdVpHajRnNUd5SVYwVkMKalhQV2puKzl0eG9yRG4yZWpqNkdwWnFGRlBkRjNwN3lGUCtaRFcrSDN6Y0lMNUxENDBoeHNGSlhRenYxR2N3UQpHd1hjMjBwekMvUGMrVzU1eEpTemp1ZXVnUUJVWVg4UDJVOUw0a3VZeU5qeXJMOXhGSGtFUVNQNEhhTDlpamdlCndVdkd6QnpnU2c5UFZVRldtMVVVTHdoN3BRQ1JlSFhBQnpLd3ArbTJpemxSRHRzUXdxeXhKbXJZeXNaUW9nPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
      key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcGdJQkFBS0NBUUVBd3Q5ait4K1N1anBGM1RCcUxKV2ZDOU9MOG5TeDVWdzRRNmF2OXJrbXFLait0dDlvCnU4TmF1VlBEcDFEbHorT3B6YW1vcTBDMFNSck9wNFpOYlhLZEJYUUdLQTRSYnhKMWNPV1hxbHRXQ08yMlJmdVAKOXpOVGN1bFh1YlAvSXRsaFJuY0RkWWNTU3psS1FoeFozRFluV3dMUDhSQ3UxSW9GVmM1TnBjNTV6d2Q4bFBIRgo0TWZwNE1oNThOS1U2YSt4N1VYTHFFcHd6U1NtdTBlMnk5TTdNSWo4MWsxcFlWNFRmUDlnZFFjZHFpUlZCR0ppCmxZTElsdm12a05DSnBqbGFDT2Fyd2VFTFRMdUxyR0lPTTgveXJHdEMxUWJxVUhxZjRuc3NuWkpycTgvaWM2d2QKVXFaaStCWEpsemEvejZnSnBBK25IUW9sVTdIZU53TFpVWXpqRHdJREFRQUJBb0lCQVFDUHdqTEV3RDhEQ1JnZgpHNmRINnJ6aEFaZTlMbDlLUDZUMksxS21aV0ppakFFVU1XM1hEait3ZGwzZzRhb1htZkRiV3F5bVlWNWVpOXNsCjlNckwwZ0NLVkZSeVdpWjhWUmEwU1h1QVhrN3kyVUpkRUQ3ZGMweTllZXlRZjN2Wlhwb0hYS2I5bmI1ZUpnNWwKQlBzNW0rMmVrMDJKbmZBTHRTSkljYUFRa0dpRjA4bFNNU2NhaVNDMHJ2cmRkYzhvTlhJYUN2dVFTbFNSV2wxWAo2TzVjQ3R0YzVlc2xmUkJ0S3dhbHNaalVTeUd5c3BQMGpLd2JpY2tvQnhya2ZvRXcrVGgzYTM5ejI2dGFLeXhyClJCbTJNZVJUMVpCTEpkOUpmamx1NjAzY3dmSjk1TlZZTjZYZHRZV2pPc0szdWtYNTBaUnhBWVAyYkNzbTFSdUoKU1hKYmE5MkpBb0dCQU1iOWRuYkpFN0dWVktkcGRlMUlyMG03WGxuQVJvTDJXRnlUdWlQenAwSWM0dHRmWDFqSgpQOTdiYXNmTkRzSkFIM2FJQWdsRThQQ2VMckFuV3d2UERDZzByMkdDTG9ZWk9VM0RGM1NjZ0ZtVVRzQllWZGRqCkN6eFJTSDJEcm0wSmR2NnVSR1JSNHpHcDFMK1dzNWJqRWpkaHpkRUU2QzJDL08zbE5BeG5sbzE3QW9HQkFQcXoKOEtsa0MvNm5YWnRSOStSV0xJb21SKzc2dG5hUjB6SXVyY0FQSExNcTNjUC8yUzBneHBGd1oydmpGRHhQMkJjMApydXlNUXV3bS9GSlJtT3JKL3JaYkFjK0hYQ2JOUk9FdjhNa042cjJINldhelBZOWRWK2ZmdXZiR1VJRkEzOGVzCjM0dEVpQ1ZHMFZSZnJpOEtHaXQ2TmRKdUwrMlVkVkp2VVRiWG1RcDlBb0dCQUxvSE1yeVI5c3RKNDc0dXBZU1QKTXV3bk1tbU5pMTNibDNmVTAydlEyVWpCWUlQZGdYR3JrdjV3K2o2WHdYaHdJZm5aNUsxdHVpSDRmNFZIQmFMZwppV2o4K0FpY2Y0bjJBdEJqMW9XNTJYUGxaa29EU3h6MUJ3ZjRwV0JSdnJ0STRlbnVXUm5BUkRtbG43TU0zQS92CmNKUTk1di9GS3BtQm41dDNiMVU1Y2xJSkFvR0JBSUl1VStheDArU3RKZGRVYmdPOGw2NDVDSnRZeHN5MUZsVDEKbGpXbjQwQktIeFA2MDl3eUs4b3o4eEE3dnpNK1JyaHVHL01yTmtrSVNYZTVkVTFlREl6R254OFRhOCtlUVlrcAphc0FNSVB2QUNudlEwVU9UdGVUcThWdlpTTTZGVUc2UUh4aGpRc3NRaGZ4cEhyckFaU3gwYm1SUjRVTmVGcm55Cm9kcDNnN25GQW9HQkFJNnJxTHZtazI2dC9YSVdQWDJQdDVHanFqTm5zQlllb3QrNXU5a0c5S2RmZEtRWExFd1YKUlA1OFM1Y3N2RkhhZ0IvMnV3WUFaSU9GSXdhdTlmVnl4TU1ieFpvSWg0eThhOEpqbmp5QzhWQzhlelZvalJXRQpvamhnWlNHeGRjdWVVNDFBNWhXNXQ3OXVVT0x2d3hTajArY2Q1ZndqZGFRZ0VoM3cxMm8zYUQ0MAotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
  additional_trusted_keys: {}
  cluster_name: me.localhost
  type: db_client
sub_kind: db_client
version: v2`
	databaseCAYAML = `kind: cert_authority
metadata:
  id: 1736815462678282000
  name: me.localhost
  revision: fc0e88e6-8dac-458c-9dec-f8cac14a660d
spec:
  active_keys:
    tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURpakNDQW5LZ0F3SUJBZ0lRS09YVVNuWDVDVTI1TStsY25wKy9QekFOQmdrcWhraUc5dzBCQVFzRkFEQmYKTVJVd0V3WURWUVFLRXd4dFpTNXNiMk5oYkdodmMzUXhGVEFUQmdOVkJBTVRERzFsTG14dlkyRnNhRzl6ZERFdgpNQzBHQTFVRUJSTW1OVFF6TmpJME5qRTFPREl5TXpFNU1qTXlOall3TURBNU5EWTNNakkzTVRBeU1EZ3pNVGt3CkhoY05NalV3TVRFME1EQTBOREl5V2hjTk16VXdNVEV5TURBME5ESXlXakJmTVJVd0V3WURWUVFLRXd4dFpTNXMKYjJOaGJHaHZjM1F4RlRBVEJnTlZCQU1UREcxbExteHZZMkZzYUc5emRERXZNQzBHQTFVRUJSTW1OVFF6TmpJMApOakUxT0RJeU16RTVNak15TmpZd01EQTVORFkzTWpJM01UQXlNRGd6TVRrd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFEY1VCcEQxSkY1TC8xbmZ6YkF0Q2kvLzVvUDREQ2tNaFlhTlVSbXlWYnYKOEc3Z2Q5YitVVm5xaHVDcmdhRGM5a3pYbE5aZnBEczR6WWpTSlM2bk5BK1RaTXplcmdNbjlhMnFmZEVwZnNXcApPRkxBVzFqODFGZVh3dCswTy9aM0Q0cXVGdHIyRHp0R1plektkVWhsaXNQUGRXejJqUW1kMGJXNUZUTG5JSE1yCnhHRVcrN0E2YUdVbnh3b2M3aEdtKy9VOS9wMDNXN1FMQ3JEbWdjYVkxeEozaGpMWnN3WUhwd0UxbFlqWm9sc0IKNlRyVkxCZkJ3UXd3TXlxeWdDVXZvQldyMWRia01BV0h1dFpXWmdhL0ppdDAzT1J2Mit4Mm1GRUo0R3NIZWJrSgpHZUlXclRjbmxxRThwU2tjUWxmNWh1a25Ob0thamE5MkcrL1AzaitTanpuVEFnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQnBqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCUTNVakNxb1crYUhFYS8KNjNQdXBUeGJONHBFSVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQXFvWmo4QXphOFRsUEo4dTJZY2RSYnVscwpZSHFxaEd6bEpwMFNtUTlMR3diVjZVeGI0UHNBSEw4WmRWOUxyN2RNems4bWFwSC9FRURhRmpzQnB3N254TGNwCjhwVHkwbXJ6Wll0elRQYUw2dnQ3M2JHdnZ1bUsyVkxmTmh2OHBLNXBVS09UdXRHNVNPN1pzUVdJakVuQkpvMnkKcVN1MFNCTzM4SlkzNkNtRjVpckhFdG5HVHBMZVd1dWdyT25kRmVUdURYakMveUd1MkNsSzBnRWpIdVlVOUIwNgpMSUxzaU1iSHJqY0ZiQ1hiVjRCVXNPaXlMSFluV2Zna1NKRzk1VnNZRkRUNUxKdm82cUY5TE1kdzlEVW9SL3o0CkVzYktMT0I4b2dOVm92NWEvSFlhVkNIYjJSeTU3TDhWNEJLRXJoNzdJT1pZQ2RCZ25yRStmS1hZQ05JT0NRPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
      key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBM0ZBYVE5U1JlUy85WjM4MndMUW92LythRCtBd3BESVdHalZFWnNsVzcvQnU0SGZXCi9sRlo2b2JncTRHZzNQWk0xNVRXWDZRN09NMkkwaVV1cHpRUGsyVE0zcTRESi9XdHFuM1JLWDdGcVRoU3dGdFkKL05SWGw4TGZ0RHYyZHcrS3JoYmE5Zzg3Um1Yc3luVklaWXJEejNWczlvMEpuZEcxdVJVeTV5QnpLOFJoRnZ1dwpPbWhsSjhjS0hPNFJwdnYxUGY2ZE4xdTBDd3F3NW9IR21OY1NkNFl5MmJNR0I2Y0JOWldJMmFKYkFlazYxU3dYCndjRU1NRE1xc29BbEw2QVZxOVhXNURBRmg3cldWbVlHdnlZcmROemtiOXZzZHBoUkNlQnJCM201Q1JuaUZxMDMKSjVhaFBLVXBIRUpYK1licEp6YUNtbzJ2ZGh2dno5NC9rbzg1MHdJREFRQUJBb0lCQURaeTRacmovVFFMUlVCLwo4ME03QTFzNFM1WWkzVUtuVWtrVjR4cllKZEZWQmNJYVBCdE1kY0Y5cGljYytXbkN3WWtDTXQwZVZMaWNLM1ZzClZSUmp6SG1zRHVuMTdiZkJnek5BdHlIZlAvQ3JoK0FjYzJqQS9najIwNXpTdVA0QjdFOU1QTDlWVWx2NnNzUHkKcW5yV0NjRExENnY3ZldYd3YwM0h6SFhNMGtuOVlGUEpwU2JlMDR3aExMV3QrdFYyaGxzZytoSjA5RkFDdjVibAp0TWxwcG4zVjBSNS9CeC9ic2V1MXh2NGZ6Rm1aRGlkbnFjUUNyakFRQUV4aHYxNTVzZGhKbDFldnVYUGYzZWg2CnVSdVhGMGxnMnRyVnBOQ1h3TFVSRDNXWGpxdm5XUzdqTEd3SEFKTEpnVGFnMGk4SEM0YWRkcVdLK3R0U3dwU1IKdjd4aUhPRUNnWUVBK0dOeTIzdHZSRzFlRVRNeCs1MnZNRHFZRDduM2NpaXRzQWpJTlYxa0Uva2ZhRSthYk5VcApKZUZXNzN2Qkc0RU5SbitLRWphdGdDMzJyd05jK0xwd2RkNUM5bVpLVFdqcjJQa1dCNEFmeTVQVVFIVm1DTjRRClQvRUVsM2k1cWdmbGhvWm1nQlVjMFl6dWpGcW9PVXpMZ21lampNR2YzZlhOc3JGSmtZeUxrTU1DZ1lFQTR4Qm8KVGZNTllqNENtbkpNV1lDb0ZJa2ovalVBbzd4cDRCTlZCTFJlanlzdVpmTHVVekpxV3p4Wi9wb1I4K3Bxd3BadwpHd01jZlZocmk4Q040VHlCd29NUGFkb2k5b0sxbkJPcVNMUm9qMEdXTUtLdmJSMWw5R2lCZVhUZ1pHenpHSWQzCmpJVlh2YndhZ0o3Y2VVNG1JRjlOdG5lM1JJTWtEa0JSOGMrTkliRUNnWUJWSDkveEVEQmx4d1dCNTRXdHNiQ2sKV3JCYVUyVldIbExJRFhwdnIzM295bXZWRjlMWWtZVDBrbkYweVhpNHNGV1lYNFUyRUw4Tk9yTmI3MDhoZnVPagp3WFE1ZFh6cFlwZlJXQ3dRamZ4WGpHWWxZUmFDMjNmRHJkbmcvMkxCdnNzT2Uya05aQzdvTWVCZkFZSzlnSEFPClZPNWNBcytEQmdaa3d4VnZhRGM4ZVFLQmdRRFVUOEFudXE3MkFHTnd4SlR0VDJaYUpVMVpZWGZpb2NjaHRSSFcKMzB4WGRCbmpTNzVhWHBhaURwRmJoZlpwYXZRK1ZHb29aOFZZMHJka3Fqdy9zZExtN0tNWjU5U3ZTTkxGU0lIOQpqMnNCSUdOdHdJQmxkNHFnZUtNdnpRQVFCdXRiTVRld1ZmSVB2L1hMOUQ3VTBpVEdPamF3K2NtTUwwOGtZREgrCjk0SFFVUUtCZ1FDb1dQb281TkhXU2IxbEpRcjBWR1VUemtKQmp4RnpxTDNxbWxyMEVUanpzdEVDSy9EejFvU2oKYW1leG8rd0RTb2ZHTEUxZHBMYkVPS3BnaE84QllvZUxFdW1LNGh6dm5CaEJOQUpYTzNqZkFTK0NISW1BZmN4bgpMQnpEZzJlR0JrNUpUcHM5bjNkcjVvSzZPNU8rc3VlSDN0TTN3RW5Vckx2VGEwcDBzQStmcVE9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
  additional_trusted_keys: {}
  cluster_name: me.localhost
  type: db
sub_kind: db
version: v2`
	jwtCAYAML = `kind: cert_authority
metadata:
  id: 1736815462676799000
  name: me.localhost
  revision: 01d66f36-8bc8-4991-8731-35c961f6bccd
spec:
  active_keys:
    jwt:
    - private_key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb2dJQkFBS0NBUUVBdjc4Y3pqZjgyVEtFczN4VGJOU1djWU53S0p3ZUVZakF1M2RVNldGSC9yQ0ZoOW9ECmRRSVhKdTU2UmE4Uk5xenNpcTl5WGtDKzR3K2pyS2dXRFVtWWgxejJaS2lnVDZEVnFBVzNYVzdlL2pUZmdRbkIKYzl1dENINlVJYUxUYnQ2RmlUM2dCZ2NnK282aG82bjNVT0dXTDBIMXQwWm9sTE9OWDBrWHMwSTZvQmw4UEl3eQpzVzdzeGpKQXplc3ZDeFJCV2VoK1doOTRIa3ZTWGZuL2pVRUJSNFFCY2VSQmxxOE1nWjNkRTNWOWVHRzBEdDFhCkpQSUx3RHpyTlJLZ1d6RnhHeUJMQllDcUh2Vjl0aUtOeTIzcW5weS9SQ1FoVkpJM0pHRGdteEhZTmtuZ0NJL1EKL2VKMEFWc0tkdm45NTY5YVllUndCSFVmdXg2WW9EZGg2aDc3RlFJREFRQUJBb0lCQURIR3RmNmVzQ2ZlSW03Sgpwb3FKQVdrRVd2aGYxcnBzaXNQZnJZNU1MN2xoTDdqZGtxb3NXY0JFaGo5U3ZDQTZjY2xxMUVDOWhCQkR2aFNUCktlNVhIWjUrTm9SWTlnelZ6c0VvZ3JwaGpzZmxCK1JpbVBLdm8xS2lNV2d0OGI5RlN0c2UwZW9lcmFQOXBONXMKd0FRaUc2KzI2c2VpSW9IL3ZvSnU0aFVwNnpnbUVoR0l4RW1iNHpaN2NzM2RVZDN2dTJ1UlNmRnljUDFGRmphNApXZVlZK3RGUHBmVzRqSjdFSkRucWlIRmFXQlRRTEdIQ2hYZFdpQVBWM1kvZGl3RVAydm5lYjhLVHBzbGU5SlA2CktoWjAzYVljdHdsWVEzbXV2NU1FMkNVTW5nUXRHam5iYVFWTE1POUV1UzdJekxUQ2V5d1piL09sYW1HaVhjbEgKSkFtWlNnRUNnWUVBeXpvWUJ1TzhUZjBCdFJ1L3RvOXVCQ0QrdncwUmExVjhXQWxJMGpiL2FPRkUzM3o3K2RrKwpHaWZ1MDU2aWMvd1JaVjgySXpiMEdZemh1MC93YmpKQmdRVFJpMHJub0wvQ1JBd0p6c09tZ3lMK2JReXdOUXdUCmZqenVnV25HalRWQ0RwTW8rcXlxQ2RETXNEVDlMTmJBZjRFL2oyWWNGb2NhMmV0UmN3a1dMb0VDZ1lFQThZblcKUkdwT3ZTVlVWc1VWR1d6ZEtBK3dGQWxTTlN6K2piclBTdUlUNWR2TjVjTUlhb29wM1A5UUxGcFFiMUxQTXMzeAo1Qmltc2tvRTdnMVN5U0sxdEVOZVBQdUpLOWhPQm5sM3RYZExsK1B6ZU1CNzlnRXEyRHh4V0o1VW53KzY0QTRiCmUyZmt1N1dJeXhjSEdnV3N3TTZNU2hHaWd2Y2ZOd3VkOFNlMDZwVUNnWUJKb1FLVGZHNzgwbTJMOEVIRklySDUKVFByK3ZQMVNwZVltL3pZaTgwb1Y5WWUrY01uWis1dEVYck5vZUZEak5MQVl5aVlUSEJYVUsvYWNwcG0xVXYvbwpmcFpzb1BiS2hxOGJlRUVWYUUwcnRjSDRRR0NXMTRrNGMxcjJDQnluakdRaVk2NjFJMWwzdE81ejZMN1JQL3orCk5SV1NIcXlPZk9SOWo0UXk2VmZnQVFLQmdDTUt3RTlFclErNzdyUjMrMHVwQTV6Z1NjZGVZdExjS0VJZnJCdE4KR1YzcnViOXZ3RFRVdnFZVlZHaGE0ZmlFcHhMVDFoZ2xpMm1xVzNTOThoakVOR0JtdGJGYlBOZGpsazVTS1EvbQpzc3ppZ1Z3dmNNeUw5czlRVlpGcHh4VWNqeHdhYjlwRGhHZkhPb1ZjWGVka2sxK1ZsN3pYT2lDT0FiVld0aDlhCmgyRFJBb0dBRzFCUFdZeGE5ZFFIUnhhVEhFSHFyaUpvN1hTNUxRbUdQN3p3NUE4ck1seldKTFJQbXhZZjAyZmUKNGhoNG0xSEQxVzUzSmRpYWlHZlFxUUZJSGplelVNSFJEN2lMcURJemNYUnppWmZiaXF3dzYxUGt3bi9JRE1vWgpOTjJnekFhb2ZnM0F6UGFXZHZNV3Zka0RnZThuVy9VWUtGanJZZjMrR01wSG4zWjZUdlk9Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
      public_key: LS0tLS1CRUdJTiBSU0EgUFVCTElDIEtFWS0tLS0tCk1JSUJDZ0tDQVFFQXY3OGN6amY4MlRLRXMzeFRiTlNXY1lOd0tKd2VFWWpBdTNkVTZXRkgvckNGaDlvRGRRSVgKSnU1NlJhOFJOcXpzaXE5eVhrQys0dytqcktnV0RVbVloMXoyWktpZ1Q2RFZxQVczWFc3ZS9qVGZnUW5CYzl1dApDSDZVSWFMVGJ0NkZpVDNnQmdjZytvNmhvNm4zVU9HV0wwSDF0MFpvbExPTlgwa1hzMEk2b0JsOFBJd3lzVzdzCnhqSkF6ZXN2Q3hSQldlaCtXaDk0SGt2U1hmbi9qVUVCUjRRQmNlUkJscThNZ1ozZEUzVjllR0cwRHQxYUpQSUwKd0R6ck5SS2dXekZ4R3lCTEJZQ3FIdlY5dGlLTnkyM3FucHkvUkNRaFZKSTNKR0RnbXhIWU5rbmdDSS9RL2VKMApBVnNLZHZuOTU2OWFZZVJ3QkhVZnV4NllvRGRoNmg3N0ZRSURBUUFCCi0tLS0tRU5EIFJTQSBQVUJMSUMgS0VZLS0tLS0K
  additional_trusted_keys: {}
  cluster_name: me.localhost
  type: jwt
sub_kind: jwt
version: v2`
	openSSHCAYAML = `kind: cert_authority
metadata:
  id: 1736815462675350000
  name: me.localhost
  revision: 64116dbb-d9b4-4765-961f-e126750732f6
spec:
  active_keys:
    ssh:
    - private_key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBNUhGRlVSYWg5QWpqRzBqakJIKzhybFlNaUN2M3Q3TDdPRVdhNEljTHUvWDdqSTF1ClN0YmsyVHVtSkhqU3ZwREY3U3VIYW5hK0UzbFdIMXM4ZzJPTXM3cHA3VWc2eWI3ekxwRTRSMFlGM1RnSkVidlYKeFU0L2JVa3NvZ1QyMDJwZVd1VkZaYUI1b3NVeFhZWUJWaFVFREM1b3NBd2wxSlNGaXQ5SmgzRWhXeXRJMnlMdwpZTUwwTUlWZVZ4M2lyZnNTT1BKdXhXSkcxVmVST1VUUC9sV3A1N3hqZStEc3dnZVFLVnhxSlNJeGtvU2ZpN0pBCk1HZTlSZGk3MXZnSnhOQWJhYmphQ2lpQWdZMU9Td3FoWnNIVGZ3eGpTSDZ6aG9OZE5lTEdGRks1WXB6UVROZzkKekp2dkRpalpJbHkxSmUzbm5CZVN0U2V0YWlGc3J0L0JCSVN4Z3dJREFRQUJBb0lCQURKTGtnUlpaRXpUVEJVcwp4ZmF1blA4UktPOHVKdnNGNS9PcXQzK3BtL2JGSUo0QVlZRU9zUkgyNVF4d29ZMmRXRVp3Wi84VHA2T24ra01yCkZqYWpTMDRpdzhHZlBubytsVkh5WFI2c0Z3eHVrdWlabjJZeVpScU5tc3NOSnI3RFU0VFZwNkxKWXg1b0pnYysKUXJzT0kzYi9ITU50MlVKbGRNVnZoY1BSTDVQZm1EOWpsalZqSTk5dEgyU2RINzYwWG5wZDFqNUlBQjJ6RnBWNQo4YkhLTzJpMmM5Q1pleGZkSENrazdLOVpIcE1PdFBCOTRZMHBoT3NGTTUwbzROa0twb1BRMTRKRjg4U2RzbGg2CmVLQm1PazhQVExKUjE4K0wybEtIa1E1OXFpelFxdnZNclpuNVFYUXJTRkJFd3k5cE9xTm5wVGJpcHI1UlpHWFcKY2p3STFVa0NnWUVBK2FxUWlObVdzNDc0eUxUaEczYmNDSU03bjVzUWhXM1lDMm1Rdnd0d1NMQ2p4YTIxVExERwptRVBDWitYb3dDNGRYcThvYWtKRTRkZU9mSHg5NmJLbmZGZXJHU1BHdmdCWldiNmVZV0sveThsV0svd2lhOVJjClB6TEI5NGZVRmJQVHo5N0pyRldBeHpsWkRRdUJKc09hZEF4S09KNng2Uk9KRVRLdkVDYU1MNjBDZ1lFQTZqemUKdk9LdUhxQ1N6Q1JaTWRBRktySnRIWjVaRi9rYWZwU1BUVGtuNUNLQUJvU2JUSlcvYnVmeEtTYURZQnU2RjA1YgpXY25SSXNlZWZrS2tDQ2YwYnFCZnhPemlrUG9BRkFVU2NCM2NWVnlWS3JuV1NqRE5MMlNFSnVLc09ZRitGS0U3CklRN05tUGI4MFdieG94RjMxdWZyLzhuK2d2RU9GTFZBV3hmdXkrOENnWUVBeEMwcDlONUVkRUxiYVpuM1o4VTEKajlyT2R0TTVZQjYzcS8vL0pKNndVKzI0UWhRRWFZWmVCamI0QXZ1OHI0V012bUdUdUNycVJTdERZcjNQa2xvMwpFSlV5ZEVhUVc2dWFpZEltVVE5dTlZbjJsQWxDWXNneTA5WG1ZOEh1L0Q2WktMVStjcE9jNU81Qzh1VWZUbjVVClZ1dHhScHdyMzZEaUN3bHdWWmgwZnVFQ2dZQVNlSnBYNnNnd1FobFJYOHhvMFM2WEgxcmJheEU3Z3JsRUloTHEKMUFjQlJuY3lER0x5dHh4UmNwamgxZGVtVElsd0xRMm5Gdk1XK3diVWpnekJWK1UrbEFiNVVIVE5XZW1IcXA2NQptS0UzV2dXcFNONU5HMndTd0twckpwVE9OQmZ0S0lteElhbTAxa1U1ZmhTdjkwQ3NBYjNxZmRORUlCNHNJOTdmClVCUFVvUUtCZ0VFR3pIL3hsdk1UQzBlNnhjd0pQNGIyWXNxY2FKVzB6VFRLbmUvRFVLSkQyZ04wMnJGRUp2STEKU3hxYkVHWkorSjlkQUlUdFBXeE5yN0xwNzNsRnpJZWlxSllOS0lzMXVaWXBuRElTZGcvVEg0Rm1rZU9zclVLUQpQbGwwdVpSbWUydVNrSVBGbmhUaXNVc0phK3kvVlhFQndGNmY2cVY0NmZGcDVoSmVPNUpHCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
      public_key: c3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCQVFEa2NVVlJGcUgwQ09NYlNPTUVmN3l1Vmd5SUsvZTNzdnM0UlpyZ2h3dTc5ZnVNalc1SzF1VFpPNllrZU5LK2tNWHRLNGRxZHI0VGVWWWZXenlEWTR5enVtbnRTRHJKdnZNdWtUaEhSZ1hkT0FrUnU5WEZUajl0U1N5aUJQYlRhbDVhNVVWbG9IbWl4VEZkaGdGV0ZRUU1MbWl3RENYVWxJV0szMG1IY1NGYkswamJJdkJnd3ZRd2hWNVhIZUt0K3hJNDhtN0ZZa2JWVjVFNVJNLytWYW5udkdONzRPekNCNUFwWEdvbElqR1NoSitMc2tBd1o3MUYyTHZXK0FuRTBCdHB1Tm9LS0lDQmpVNUxDcUZtd2ROL0RHTklmck9HZzEwMTRzWVVVcmxpbk5CTTJEM01tKzhPS05raVhMVWw3ZWVjRjVLMUo2MXFJV3l1MzhFRWhMR0QK
  additional_trusted_keys: {}
  cluster_name: me.localhost
  type: openssh
sub_kind: openssh
version: v2`
	samlCAYAML = `kind: cert_authority
metadata:
  id: 1736815462677271000
  name: me.localhost
  revision: 4416cb52-14b9-4197-bae0-1c272906db18
spec:
  active_keys:
    tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURpakNDQW5LZ0F3SUJBZ0lRSjZkczBCMXFjVWF6R3c3L2IrRFlzekFOQmdrcWhraUc5dzBCQVFzRkFEQmYKTVJVd0V3WURWUVFLRXd4dFpTNXNiMk5oYkdodmMzUXhGVEFUQmdOVkJBTVRERzFsTG14dlkyRnNhRzl6ZERFdgpNQzBHQTFVRUJSTW1OVEkzTURreU1USXpPVGszT0RVM056ZzVOVGM0T0RrNE1EWTNOakEzTWpVM05qUXlOelV3CkhoY05NalV3TVRFME1EQTBOREl5V2hjTk16VXdNVEV5TURBME5ESXlXakJmTVJVd0V3WURWUVFLRXd4dFpTNXMKYjJOaGJHaHZjM1F4RlRBVEJnTlZCQU1UREcxbExteHZZMkZzYUc5emRERXZNQzBHQTFVRUJSTW1OVEkzTURreQpNVEl6T1RrM09EVTNOemc1TlRjNE9EazRNRFkzTmpBM01qVTNOalF5TnpVd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFENGlaUXVTc3M5RlJyWjZVNGFSc3hWWGd1UzlNVlJKRDhrcHBXbFJUSU0KTXY3RXpFQmRVcGtaNWdyU1RUQ1M4TFdodEQ4THJ1NUFJZDFVTldvRkFra1NNdlJPd1hWZFJTT0ZOejZSVkJvUApPTjZBdWlrSW5lUkpGZTBCMXc2QUxlNE01RGx6ZnJMamc4MmtqamRZSnRzaFdHS1NKY2RybTQ2TVFaWE0vLzBZCk05YlFMeDU1eVJCY1FNR25wVmxCQ0pmajZDd1F0cjVZOGxVeFZscHNMbGwrSjBMR2pvTHJtcncxZU8zVGNEelMKbzdlODdFQ0dqbGYyTGk1ei9Pb3dGcWE4RVhMaW9RRjRsRWh6U1FsRDRJdGhUMGlDL3pqR0NmcThxMDF4TjY2UAp1QTk0cm9pYU5mMDNtZWJJL2IvSW9qMEgxNTNCTGZydWlZYnFoRHlHQ0JQcEFnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQnBqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCVFM5Q25hZEFMdFJQNkkKb0ZwRWtUZVRISGhGV2pBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQVJOcjg2QnJXS2E5b3NOMmFJRTBoUVNtRwpiZ2FFY1I3WVF3aW8zWUplYXoyc3RTMHQ4VXVON3FKdkU4dnYxd2F0eCtTMXhhdGdBRWlXRER1WUYzUWhaZUE3CmJMZCtELzc5YlFnNXBmb0E4UDA1UGR5cmtzalZ2SjcrWlY0SFFoaTJOUE9PbkExL3ZMNGI3SG40UXFnRDZsbHcKeFRDS3RJOVoxTVlWN1I5dUthNllpVDlCNWVDR1NCZWdQdnNlbzV6Y2p5aGdJeklTRkp6N09MR0dQT3JyUWpLUworOWJ6bjlnOHM0dHBwd0lEWUh6UFRiOFovR0YycmV3MlM5cDBQUGlnTGIyT1dKc1M5eDcvQVVOa1NHQWxiNER4CmdVc3dsWEhtSXErWUZaeDFZL21DeWlndWdQNnR0MDJjSm0vQTUrNC8vRVMrUGRpK2xqL3o2WitJLzIra1d3PT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
      key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcFFJQkFBS0NBUUVBK0ltVUxrckxQUlVhMmVsT0drYk1WVjRMa3ZURlVTUS9KS2FWcFVVeURETCt4TXhBClhWS1pHZVlLMGswd2t2QzFvYlEvQzY3dVFDSGRWRFZxQlFKSkVqTDBUc0YxWFVVamhUYytrVlFhRHpqZWdMb3AKQ0oza1NSWHRBZGNPZ0MzdURPUTVjMzZ5NDRQTnBJNDNXQ2JiSVZoaWtpWEhhNXVPakVHVnpQLzlHRFBXMEM4ZQplY2tRWEVEQnA2VlpRUWlYNCtnc0VMYStXUEpWTVZaYWJDNVpmaWRDeG82QzY1cThOWGp0MDNBODBxTzN2T3hBCmhvNVg5aTR1Yy96cU1CYW12QkZ5NHFFQmVKUkljMGtKUStDTFlVOUlndjg0eGduNnZLdE5jVGV1ajdnUGVLNkkKbWpYOU41bm15UDIveUtJOUI5ZWR3UzM2N29tRzZvUThoZ2dUNlFJREFRQUJBb0lCQVFESGtURjdPbk9YeUtxVwo3OC9YS2FKSnFncUJKaXFLelNBbXZkekxxSlJYVjF0Yml1YmtDTDhISE1EenZTZVQxZFVDMDBrTWlKcW14SXFFClk1K09CaGZHbFVPM09ZQ1VORUFoYUFyRmgxS2xoblNqeU5mS0kzNTdjUyt1bXBENk8rYzZVc2dQQlYxL2N3WmQKYkJUa284NnhKOWQrb3ZkT1lNcEZ0U1FrU0NsaWxDaDlFZCszZlFMTmZ3dzJqOEpheVpQcE1BMkZKU1k1ZFFLbAp0OWhoM3dLa2pvV3dOTU9yMWZ5TzRzSXc3UnNGdURGNHdaejlZSmhGQ1Q5aFZhejUyQkVQTEdnSWxKS0F2bFdRCndMT0JqSzcyZncyTUZ5SzM3NElYc3YwZVk3RU54NW40eWNxREl2NnBmVEFvTDIrSk9NbkxqY3FaeEJuaG44VmsKdmQrTlZFdEJBb0dCQVBtYy9mOGw3NGlKVitxVy9iU1MwTmtHUHVoYzhBQWVzU1NQdHpENC9UNW16SUxPWXR5aQpUTlR5UkpUR3NJU1UvSEliOUpqYVdBWllDd0ZZektDREZtOG9FQnBiQTZPcDZhOGlGVzRkYUU2ODlzamJMbTBtCjc0TWhFaE9PR2grSkVLUkx5dCsxaGFKUzB0d2g5TGdiVmJuRU1jZkxhNzNNZHNqYWNqU2lGRkUvQW9HQkFQN2wKaWk1MXVWV2xtYlpZMCtOZFNmY2dsSWNWRHE5eUZlNjdCZFgybnBrV3pkRjlEK3ZlTytIeUxtMVl6QjE5bmtrMApYMDJ6T3FadVF1bFdqV01OYWdVYkQ4bkJkbmtHRkx4RkIxLzhERzdBbkFvNXo4b1J2bXJPeWhWTVJGTEdQZjM0CmZPYTFuRmU0TXJmM0lUTDlNZlRPQUh2ZUxsV0xwZ3pqVEo2dU9palhBb0dCQUk5TXlhVEpLcExBQm5EdTdnZlUKb1lGMlRIY3BvNzd0MzlTVmpSM1lVOHFYU2FGdXl1TFBhangyT1ZrUUdCYUZVY2hRdEVOc1ZreU9Ed05lNzFyVwo1dkk1bGNVTHF6TXlRSzRDYXpza050VzlOaEJwaEdXMWpKdERTUlZnNXk1amllSklnTmVkWm5LaUNkdkd3cTlQClFnKzd5cmhnMkNIR1dBdEhIWG1KOHhBUkFvR0JBSVZnclJhMGlUOVV3UU1XcGdGQ0RuTWUvRGxXL25FMXZGNUkKUkx4NktQRW9hcGhrM1pEcG4rSVNMTk1ROVBXMWhyNzloYVVOMVBIRG5vV2t3YVVFSHViL0N4cmlmZERFS3ROOQpOMmUxWnZnSkYxMk9kTGxpNFlYWUlReFY5U1p2RDM4MnFIeThxVXVKV2hqRFd2N29XRnlsOHNEZU9OYVFsVm9ICkVrK3lFVUxQQW9HQUJCNmFzWC95QzVvK2FiMTgvckdkZHNLd3JIbUJqTU5zZ3B4RHU5eVBxMHVGQmVxa0xuakMKSHBwRmlEbHNtdVl6TnZqMzAzR2ZJQ0oyMnFIajd3VE1xNmRtVTZoTTRuZ0dlVU05SGtrbjEwZnVXR1dwdWd0RgpNbHBSeHU3VFBpRDZvQ1JkQ0RnN1NlZnZ2SnptWGRIRDduWDI4R0R0bDdMTmxBbkJCTHRTVFM4PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
  additional_trusted_keys: {}
  cluster_name: me.localhost
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
  name: joe
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
				cfg.ApplyOnStartupResources = append(cfg.ApplyOnStartupResources, user)
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision", "Namespace"),
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
				[]ssh.Signer{test.cert},
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

func TestMigrateDatabaseClientCA(t *testing.T) {
	ctx := context.Background()
	conf := setupConfig(t)

	hostCA := suite.NewTestCA(types.HostCA, "me.localhost")
	userCA := suite.NewTestCA(types.UserCA, "me.localhost")
	dbServerCA := suite.NewTestCA(types.DatabaseCA, "me.localhost")

	conf.Authorities = []types.CertAuthority{hostCA, userCA, dbServerCA}
	auth, err := Init(ctx, conf)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = auth.Close()
		require.NoError(t, err)
	})

	dbClientCAs, err := auth.GetCertAuthorities(ctx, types.DatabaseClientCA, true)
	require.NoError(t, err)
	require.Len(t, dbClientCAs, 1)
	require.Equal(t, dbServerCA.Spec.ActiveKeys.TLS[0].Cert, dbClientCAs[0].GetActiveKeys().TLS[0].Cert)
	require.Equal(t, dbServerCA.Spec.ActiveKeys.TLS[0].Key, dbClientCAs[0].GetActiveKeys().TLS[0].Key)
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
