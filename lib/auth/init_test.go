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
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/services"
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
  name: me.localhost
  revision: 6cc380df-816d-4c6b-8fc8-c20af75eede2
spec:
  active_keys:
    ssh:
    - private_key: LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1DNENBUUF3QlFZREsyVndCQ0lFSU5WTzBiNUxSOXk2Nm5SRGJHN3JJUzFRZ3dBcUVpSWtMZS9WVmFrd3pJZ2oKLS0tLS1FTkQgUFJJVkFURSBLRVktLS0tLQo=
      public_key: c3NoLWVkMjU1MTkgQUFBQUMzTnphQzFsWkRJMU5URTVBQUFBSUw2ZWtVSTg3U3VOYkFiWnhPbGxRUEJJWGdPVjFNcEt4UWVNQXB0MklpVlYK
    tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUNBakNDQWFlZ0F3SUJBZ0lSQUpKYkRNWGtYN3ZOR0EreFRQeGlzYzR3Q2dZSUtvWkl6ajBFQXdJd1lERVYKTUJNR0ExVUVDaE1NYldVdWJHOWpZV3hvYjNOME1SVXdFd1lEVlFRREV3eHRaUzVzYjJOaGJHaHZjM1F4TURBdQpCZ05WQkFVVEp6RTVORFUwTURBME5UUTJOakkyTlRrMk1qazBOakkxTURNNU5UYzJNamM1TkRBNE1qYzJOakFlCkZ3MHlOREV5TXpFeE5ESTJNRFphRncwek5ERXlNamt4TkRJMk1EWmFNR0F4RlRBVEJnTlZCQW9UREcxbExteHYKWTJGc2FHOXpkREVWTUJNR0ExVUVBeE1NYldVdWJHOWpZV3hvYjNOME1UQXdMZ1lEVlFRRkV5Y3hPVFExTkRBdwpORFUwTmpZeU5qVTVOakk1TkRZeU5UQXpPVFUzTmpJM09UUXdPREkzTmpZd1dUQVRCZ2NxaGtqT1BRSUJCZ2dxCmhrak9QUU1CQndOQ0FBUzZTZE92WEdZV2wyY2FCRHFPcXRRN3RyNW1JWkdKR0JFYXJWUnMvSFFvYXpiZExKK0IKMExMUFhHemQvOTNxZ04wNXJVWGhyNHVXb3pEeTMxR2V4ZDZmbzBJd1FEQU9CZ05WSFE4QkFmOEVCQU1DQVlZdwpEd1lEVlIwVEFRSC9CQVV3QXdFQi96QWRCZ05WSFE0RUZnUVVLWmhpWVRwc3Z0NTkwTXc1OGhPY2xNcTBVYmN3CkNnWUlLb1pJemowRUF3SURTUUF3UmdJaEFLcFExNWt6MFZlQThmOEE5S3RKRS9LLytxVkkyNGFpMnM5dytmYTQKVFNFdkFpRUEwWWtNZGNFc1dqTjBFeFpzVmVsRU5EdS9hcWhtUkpua2FpakJDNjFBY0Q0PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
      key: LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JR0hBZ0VBTUJNR0J5cUdTTTQ5QWdFR0NDcUdTTTQ5QXdFSEJHMHdhd0lCQVFRZ3RTcVllSWhXeTZ3S3QxVDYKUVdVSitTVkF0SjRpNkppajhac0hmWkw2YzJXaFJBTkNBQVM2U2RPdlhHWVdsMmNhQkRxT3F0UTd0cjVtSVpHSgpHQkVhclZScy9IUW9hemJkTEorQjBMTFBYR3pkLzkzcWdOMDVyVVhocjR1V296RHkzMUdleGQ2ZgotLS0tLUVORCBQUklWQVRFIEtFWS0tLS0tCg==
  additional_trusted_keys: {}
  cluster_name: me.localhost
  type: host
sub_kind: host
version: v2`
	userCAYAML = `kind: cert_authority
metadata:
  name: me.localhost
  revision: b82425a2-dd10-49c6-9b8a-3f10e3ec8a41
spec:
  active_keys:
    ssh:
    - private_key: LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1DNENBUUF3QlFZREsyVndCQ0lFSU1tK2EyVlRqSHV2OVhrWG1UR2Roem5GSVJPK0pTaVEyWUF6eHNleEJFZGEKLS0tLS1FTkQgUFJJVkFURSBLRVktLS0tLQo=
      public_key: c3NoLWVkMjU1MTkgQUFBQUMzTnphQzFsWkRJMU5URTVBQUFBSUtZYU1pUWZTUWgyQVpqN2lNSEFBQUpxU0FQWWJLL0gzRFJ3NWtUSXBDRUMK
    tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUNBakNDQWFlZ0F3SUJBZ0lSQUpsUkc4OFlqTURVUmZmOENYMnRpbkF3Q2dZSUtvWkl6ajBFQXdJd1lERVYKTUJNR0ExVUVDaE1NYldVdWJHOWpZV3hvYjNOME1SVXdFd1lEVlFRREV3eHRaUzVzYjJOaGJHaHZjM1F4TURBdQpCZ05WQkFVVEp6SXdNemM1TXpBeU16UXpNelV5TURFNE9URXdNRFU0TnpNek9EWXlNalkxT1RVMk1qQTVOakFlCkZ3MHlOREV5TXpFeE5ESTJNRFphRncwek5ERXlNamt4TkRJMk1EWmFNR0F4RlRBVEJnTlZCQW9UREcxbExteHYKWTJGc2FHOXpkREVWTUJNR0ExVUVBeE1NYldVdWJHOWpZV3hvYjNOME1UQXdMZ1lEVlFRRkV5Y3lNRE0zT1RNdwpNak0wTXpNMU1qQXhPRGt4TURBMU9EY3pNemcyTWpJMk5UazFOakl3T1RZd1dUQVRCZ2NxaGtqT1BRSUJCZ2dxCmhrak9QUU1CQndOQ0FBUlN0T0tLYklVeGppSTFIZEJGU2xVQW1LQW0wUXlxejljWVZEdlZEc0RNT1BqNFRTWGsKOHp5Q0FhYXloTlgra21lZlU3R2ZIeDBlVVMxbVhBYnl6UnNHbzBJd1FEQU9CZ05WSFE4QkFmOEVCQU1DQVlZdwpEd1lEVlIwVEFRSC9CQVV3QXdFQi96QWRCZ05WSFE0RUZnUVVKZi94RjBtMStZM1lIRlRubVJhcGJ1dVdqa293CkNnWUlLb1pJemowRUF3SURTUUF3UmdJaEFLZkU5bXpPdzcrcUx3bEFxdFFWUVBNODVFTGp2NlFSdmZrcUE3aWoKSEtCMUFpRUFoNjVJZmV4NFpwT0szYVVUVTZQWGFkODlDdmhVb3NFMzRzWU1FZHdxZk9zPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
      key: LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JR0hBZ0VBTUJNR0J5cUdTTTQ5QWdFR0NDcUdTTTQ5QXdFSEJHMHdhd0lCQVFRZzRXUVZQclNGTmR6bVFMRWcKQkMvTU5sb2JEL0dKekNGU01vOUZsV0pjZDY2aFJBTkNBQVJTdE9LS2JJVXhqaUkxSGRCRlNsVUFtS0FtMFF5cQp6OWNZVkR2VkRzRE1PUGo0VFNYazh6eUNBYWF5aE5YK2ttZWZVN0dmSHgwZVVTMW1YQWJ5elJzRwotLS0tLUVORCBQUklWQVRFIEtFWS0tLS0tCg==
  additional_trusted_keys: {}
  cluster_name: me.localhost
  type: user
sub_kind: user
version: v2`
	databaseClientCAYAML = `kind: cert_authority
metadata:
  name: me.localhost
  revision: 797ca33f-e922-45db-b531-ab62af23963c
spec:
  active_keys:
    tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURqRENDQW5TZ0F3SUJBZ0lRWGtnVThDZFo3TnkrbVJacG9GZUwzVEFOQmdrcWhraUc5dzBCQVFzRkFEQmcKTVJVd0V3WURWUVFLRXd4dFpTNXNiMk5oYkdodmMzUXhGVEFUQmdOVkJBTVRERzFsTG14dlkyRnNhRzl6ZERFdwpNQzRHQTFVRUJSTW5NVEkxTXpJeE56QXhOalV5TnpJMk16QTBORE13TlRRM05EQTNNVEE0TXpjM05qUXpPVGszCk1CNFhEVEkwTVRJek1URTBNall3TmxvWERUTTBNVEl5T1RFME1qWXdObG93WURFVk1CTUdBMVVFQ2hNTWJXVXUKYkc5allXeG9iM04wTVJVd0V3WURWUVFERXd4dFpTNXNiMk5oYkdodmMzUXhNREF1QmdOVkJBVVRKekV5TlRNeQpNVGN3TVRZMU1qY3lOak13TkRRek1EVTBOelF3TnpFd09ETTNOelkwTXprNU56Q0NBU0l3RFFZSktvWklodmNOCkFRRUJCUUFEZ2dFUEFEQ0NBUW9DZ2dFQkFMMDZCa0hXMTBpWm1NTHZ6Y01tNC9WNmk3M2EvWjNpM1hzN1IxamYKc0FDQlB5TkNJTkFCbnFoV0NHdWJPb0JWT3REemQyVUJTZ0JXYVVNcXhiUytmQjBLYllEVDVkaFN1YXd4Wk8zQwpmN0xaZWtlZk4vb1FZSjk2WlVoemVvL1g4dkZZNE5ocXBIbXJMV1VRTGpZUEZ5cVhIWGM1L3hTUkVPVm8ybnZyCjNOWXFnR2EwRUFvMTRvRUJsR20zMmNQUWNvTGRBUUhqaFJrampQYStNeXRGSmNzeFkxWTFDemM5V1hnYzZvK1AKRWp0U3dRMW9Fd3ZOVHhYbXp1UElOcTZxdVZjS1kwcDF0anBWT1R1dWpaZ1R5NGRIMkJIQ1I5d1ZxejlVRGJPWgpWVHVKWjVnRFdQQkpGcVRPNUhkbzh5QnVLOXM0RndYZDJoaC9oa1YrNitabXdJRUNBd0VBQWFOQ01FQXdEZ1lEClZSMFBBUUgvQkFRREFnR21NQThHQTFVZEV3RUIvd1FGTUFNQkFmOHdIUVlEVlIwT0JCWUVGRTZ4R09aSmFoTzUKc3FmQ3pZK0ZxTjQ3eGpWeE1BMEdDU3FHU0liM0RRRUJDd1VBQTRJQkFRQk41V2JLdVRzaWlvUlltS3BlcGtiTQppT2FCbGJ4ajZtOTZDVHZpczJSeXlmUU1qSXduT1dYbUtMMHpGQUtvZThDWVk0UEJFWEtuZzZBd0NNVXdzWnNPCkZPMFE2Q21JUzlwQXdVZURIZ3VrN2x2dWQyeGFvb0MrazY4OGtmUlZhRjVPTW5WaW4xajBJY1MwM1d1QlhSckwKVDVwODFza3JOaG9Gd1BtdnU0RkZmU1lHb1YrajQ4NC83Y0FOY3J2c0FFdFBFdk9EdzlUVG5vb05zdUoreVhaMAoyVW5LSG1rOTFCeGdZVDVHUE1CVkYwQWxzenhjcVN0S2hBMlhrdzBCY1pnTXVBa3BYSXJRbUZNNGh0RnIyY0NOCkpTTk1mVUpsZVlXLzJiNmJvbENZTUliTFoxeldaaDBVYTJ6QjlOME9wNE1aS25xRmhtc2JVWFFOa0xGd1FEYzAKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
      key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcFFJQkFBS0NBUUVBdlRvR1FkYlhTSm1Zd3UvTnd5Ymo5WHFMdmRyOW5lTGRlenRIV04rd0FJRS9JMElnCjBBR2VxRllJYTVzNmdGVTYwUE4zWlFGS0FGWnBReXJGdEw1OEhRcHRnTlBsMkZLNXJERms3Y0ovc3RsNlI1ODMKK2hCZ24zcGxTSE42ajlmeThWamcyR3FrZWFzdFpSQXVOZzhYS3BjZGR6bi9GSkVRNVdqYWUrdmMxaXFBWnJRUQpDalhpZ1FHVWFiZlp3OUJ5Z3QwQkFlT0ZHU09NOXI0ekswVWx5ekZqVmpVTE56MVplQnpxajQ4U08xTEJEV2dUCkM4MVBGZWJPNDhnMnJxcTVWd3BqU25XMk9sVTVPNjZObUJQTGgwZllFY0pIM0JXclAxUU5zNWxWTzRsbm1BTlkKOEVrV3BNN2tkMmp6SUc0cjJ6Z1hCZDNhR0grR1JYN3I1bWJBZ1FJREFRQUJBb0lCQUM4MnF4a0NZZlRiWGlKRgpjekdlSW9LOWNPQ09JM21oZ1dHZUNNOUVBTVlmZVlGeW5uMUg2aTVXU1FPUVY2aHRtNTlISUNNemp5TkdiRDAyCkR0NXFLTTJXTEh4WVlxRDNBeHpUdGpzY3JJQVRnMDhiaXZ2NTJpSHdpQlRydTBqb3VOVS9OOXJId1FJYWs5a0QKa0lRc2Y3dEF1VGxtWHg3aWt6U3FWTmxXb0dOUENZek9Bb3FZcXlTaWhEc0pLZ3N1SVFOV0lPVzE1dGxmb0NsQgpTTkF0cGxlclBNbFl6T2FjaUZVWkJFbGVOWEp0VURwd0szOGU5N2RMclJrYWRLeXhSVTNVTTE4dFR5TDZrZ2hMCmVsT3JBWDE2K0RFdm1PeDhGQlgyS29tak9OL0pORVZUN2lkcndyVnh2OU1PTUFpQjRDMms0a25jbUxIVE5DTE0KelJmOHY0RUNnWUVBNVg1d0VXVlZ0WFhNek02VVZCZ1ZwMGZmNVlPTTNTV3FoOHE1OGQrLzVBS0ZZSjZ5U1BpLwpoaVJvZ2xXc2NheXV3UVhTaHVMV2ZQVHJxMmVQNFVlcGdEa0g0c3NiamFwODFTMFZnN2JJZ3ZrdGxOTjdFMkRJCnFhMS9pNzBBQW9GOFNNY20vRE00N3RiTGxxS0ZwT21GSjB3dk1zUkh5aTJGM3pRaDB1aTFTQThDZ1lFQTB4VDgKSFZISnl1VHEvMERBdi9kLy9RNEtFUW5CS3ZmaFV5K0FOd0NGNkFhRW5USnAzTTRmNkpmUkVRUytFd1ZVVFk5WAoxWkNBdTd5N2NoQmtORG5TMmZJR2ZiQnYramVGWm9SZDRkQ3RwUnB1bHJka21yNGFaZnZnaGZsanJkdVZrdEM3CmZtQjZORTl5TURMYVBWT3Z4VEZGd3ZYMHRRUFdmYVNpL0Z3NVhtOENnWUVBdUtJT201QkJjbXBCeUl4eXZXMWIKRG1nKzg3SHdoSU1uUFhTV1FNaFk0Nkk3bUU1VTlXeGEraHNVa2JkSHMzVFFhNjY1ZjVmRUpHZ1BxcWo1RXEvSwo2TVA1V2pjNkJiR2lHUWZhaFV0cTZpUjZ6WCtQUnpuWWR0cUZBUEdmcm1ScWowcmFUSkVSUHVaRWlQNWNNeDlFCjV5YmQyaVFiOWNiR0s1c1BrMVZ4YzNVQ2dZRUFybkVzUGNyRzJyKyttYjVZelF6c29DUkhHM2VWUlQ1ZjM5QmsKeEkvUkdrU3d1ZnpjMGhjaTlhVHBxWWZpMFhOWkRWUUdRYi9QTTlld2pYNlFZVHpjVFRPZ082VmhsVWJuSHljTApNMEN6RUx3OFlxQWpLMk1xQzloUjRFYVBJekpTZFdlOVc1Njl2NWRjaGdxd28zZ1N6Z04vWkxUQlRBdGs2cWJ4CjcxOEVKazhDZ1lFQXZGckFPc1VJdmh4T0FYYk5mMUxwbnlabXVORUg3ek9XZ1FNM1ZlNG9iT3hqenVHaGs2RFUKVGhFdDg0emhsMk5LeFVacHRwdEJLRkdxM3B2dGc2TElqbEtpWmNPUEovbHcrTVZPb0U0S1Y5U0F3Q1VPN3o4eApDbTlzM05ZQTV6RWZRdGg0OEt5NmdBZVhZUHlkMWZidjl2QXlwM3FQbEpxa281SG5peHNxamNvPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
  additional_trusted_keys: {}
  cluster_name: me.localhost
  type: db_client
sub_kind: db_client
version: v2`
	databaseCAYAML = `kind: cert_authority
metadata:
  name: me.localhost
  revision: 3b019bb5-051c-461a-a7f4-e335fc031bf3
spec:
  active_keys:
    tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURqVENDQW5XZ0F3SUJBZ0lSQUs5OUdYVGFmVEhNNWN2cEpzK0xhTkV3RFFZSktvWklodmNOQVFFTEJRQXcKWURFVk1CTUdBMVVFQ2hNTWJXVXViRzlqWVd4b2IzTjBNUlV3RXdZRFZRUURFd3h0WlM1c2IyTmhiR2h2YzNReApNREF1QmdOVkJBVVRKekl6TXpJMk5EUTFNalk0T0RBd016RTFOekl3TWpNeE16TTVOekl3T0RZMU5UVTFORGMyCk9UQWVGdzB5TkRFeU16RXhOREkyTURaYUZ3MHpOREV5TWpreE5ESTJNRFphTUdBeEZUQVRCZ05WQkFvVERHMWwKTG14dlkyRnNhRzl6ZERFVk1CTUdBMVVFQXhNTWJXVXViRzlqWVd4b2IzTjBNVEF3TGdZRFZRUUZFeWN5TXpNeQpOalEwTlRJMk9EZ3dNRE14TlRjeU1ESXpNVE16T1RjeU1EZzJOVFUxTlRRM05qa3dnZ0VpTUEwR0NTcUdTSWIzCkRRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRRE8ybGRZbjhIc3ZNWTkzZVFMSWRoMUk5WmJtWWFlZm80UHJJRTgKNXA0bUpSUnVMMDg2Y0xlbFZNZDc0K1h3RmlSOUpLNlY1ZHBYVGw1cFoxTVBEbjh1cUkvMjAwWnlvVHlad1ZWUgpISUdwSjFqTVRZbjBQcDE0TC81Uk9MbXAwVTEzL3hRVDd4a2QxZnA4QVdzVzFYYnloc1hYaEhkMUpraTRFY0djCkJtSjc0Nml3aGxyUVNZcTNvSFpXNzZjdVVaM0g2bU9QVzVZenl1cUVYR3J3SUxneExZSVZNNXlxTUpEbHdiTU8KMzlBTGE4bzl1U1ZiaGJ1QjlaeG9BT1h4ck9ERGJSN0RuVTc0cEFTQm5pV2g5aTFEMTBBK2NlMEQySVNjTFcvWAo4MXpKSzVGSXRZSldSVDQvZUo3MmNMUGUycFRlSTMreEFCNW9tZVBCQjlrZjlLanRBZ01CQUFHalFqQkFNQTRHCkExVWREd0VCL3dRRUF3SUJwakFQQmdOVkhSTUJBZjhFQlRBREFRSC9NQjBHQTFVZERnUVdCQlFacW9pb3paYVoKMTVOcFMrVjVUV1RUNnY0S0V6QU5CZ2txaGtpRzl3MEJBUXNGQUFPQ0FRRUFPZFhnYk1XWUM0TFdmMnRtNUtyZQoyM2J5MlgxWHZYQ2U4L1RGdFVsMUNhdHVrV2MwY205cXUxckxiRy84ZUxCWjA2cUZSc1VTSEUxamEyRUtkREU0CjU5Y0p3bE5DN1hCMjRTQ2s1QTcxNGtpSDZMQjAxRk00WGEzWjg4YnZCSHV2ak01Q1ErN1lSSjlGZFd6YjFKK3UKREF3czI1V0haWTFGV0ovUGt5eTVycjJNTWVvS3dWMGRNSjJJWlZVMmpGZkVhYXhLWml4Y29EbG5KclIzWThEMwovUVZRckY5d0xkQWVENkExTlpjbnd3Q2F0MWFoOVA1cWhQdGlnc2lsOVM5dkd3T0JyQUEwNE9iZXQrVFNjd0tnCkVQQWhFWVlkRWUvRzV6MVZUQXBCUCtzeXRWcHdRZXQxZDRlV2VvVW04QU8yZEVuQWhhM0FvU1pmQjFTVVgxTmEKb3c9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
      key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBenRwWFdKL0I3THpHUGQza0N5SFlkU1BXVzVtR25uNk9ENnlCUE9hZUppVVViaTlQCk9uQzNwVlRIZStQbDhCWWtmU1N1bGVYYVYwNWVhV2RURHc1L0xxaVA5dE5HY3FFOG1jRlZVUnlCcVNkWXpFMkoKOUQ2ZGVDLytVVGk1cWRGTmQvOFVFKzhaSGRYNmZBRnJGdFYyOG9iRjE0UjNkU1pJdUJIQm5BWmllK09vc0laYQowRW1LdDZCMlZ1K25MbEdkeCtwamoxdVdNOHJxaEZ4cThDQzRNUzJDRlRPY3FqQ1E1Y0d6RHQvUUMydktQYmtsClc0VzdnZldjYUFEbDhhemd3MjBldzUxTytLUUVnWjRsb2ZZdFE5ZEFQbkh0QTlpRW5DMXYxL05jeVN1UlNMV0MKVmtVK1AzaWU5bkN6M3RxVTNpTi9zUUFlYUpuandRZlpIL1NvN1FJREFRQUJBb0lCQUgxbTZ2c2taeG1SWEJHWAptcStSQmp3RnpPZGRUTHA3ZUw1UjAwdkxkK2NpSloraStNSXlJWE9PMFJ6dmphK2VqT0o5UVlaSWdiVGFJdXg5Cm9tSUhaTjB4ZlkyaWlodm1XZW5ReGx0VkQ5b3ZxMnE0TzBFaVVLN1RVYmVGenpEL1hacTR2a0JUZklPVS9MVCsKMnlCTnF6M2VyTVE2WDMxYkIwem9IdHJyRi91SWNzNHNnZXFOTWFtU2JUY2I1ODRSbHhUNDA0MUdlWWdtNlZCRQpHWnpRSjJESXlQbEExZFhGUW5lcHpaaWRqQmdJMWJKS3lTMzVsekdjWVc0aGgvbVFhUVp5ZmRuS3Z4QmlsdUlBCjBxcnAvTEQ4QVp6RWxONVNMb2JtMXVEZEIrRW1RNllKbDEvdm5iWEY1QWFXSDN6b3U1OHJDcGFudDcraVJ6OGwKNFhuWjl0VUNnWUVBN1dPajhMSDBYVnFMcDMyTnNrK2xnNmYwRSttVUZnMU93eWZGSEJPdlpsRDQ1T3ZQNWx6ZgpsLzNoaUJiZEMzNHNhWU1HdW9GZlRhMjJiVHpwTXl6eXFhZVRhdy9HS3psR2Jlc0dIRk9Xd3F2U2Ivd2tWTEUvCkZocTZGSE1vWGp0aTE4eFhjRXdJQmVQaHhVZVkrZkd3V0RTREV1c3NZdFFxQjh1ajBXaDlYUDhDZ1lFQTN4SFgKbUhpdjFWLytPMkNPZDdWUkFWaThKTS9JZ0NQTk84L0o2SWRYTkNwKy8wNW10dEh0QnpyVFN0cXBGMlBSUWJEaApjYmpuYk05TGJTTVNKTHNtYndTOEhtT3MwSGQwSzU1aXdqRmFxNUdOT0xBQmlrOGMrU2Naa0d5YitxRHlGRU9UCkMzMThFSEg5SlZ6L24zYWxHZmRGWG9xWkZ1RXJvQWJCTkFEZlBoTUNnWUJsZHkxZmQvQ201a2pDOGx0YVY4aTcKR1ZLdUlDeDNzSUIxMGMzaVRsZXVOL1hxZ3hCOXVqeW56cEJUaHRJOFUxWFFVM3pRd3ZObFZGYWhJbVBheDkrQQp2R3U2V3llczJmSk1rU1F2ZjFyMUlsUDBJYVcxdlh6bGljNzNackZlZGF1dDZWMkdWamtucTF1WTR4MXoxK1kwCkRWM28vRFFnbWViTkpqR0RGRkpoS1FLQmdEaG1VSFp5ZlRLYjFMRzZsZ3JhUXlMdUJwUGdIVGVZMWJrN3JqY20Ka1B2VmlzcU9UaFlIT2NETU5NUUdTUjVxMUd1aGh6NnptMys5WWJxMFZWQUlLWTJFU3ZQOEM2T2hzRE9mRmlVMwpTVTk3dTVNTG5UZ1ZES1JLS0lLRmsySm84d3dBa2RzajNReGpaYmZlclpycDZwQ0lIbmZxM3c0VDNHM1hoMTNZCm9wa1ZBb0dCQU5USGlsZHVuWXBCTXNFVks5TFRWSm95TlNOTVdBbGFGaWlwZllmWm82M3RyQzgxK0FtMHFDQW4KZTF1V2NwRFFkQlZzbGRnTWV2TEcwOUNBU05rRjN0ZUJqWEsranNzWTRWMllTU1Bwd0dyWWhjZHpFMFBIZEVROApiMGllSGJUMGRZZ3hjWmczbW9pN1BDZi90d3Joa2lDdllIMlBMR2hzMkduV1JzSEQzcHY1Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
  additional_trusted_keys: {}
  cluster_name: me.localhost
  type: db
sub_kind: db
version: v2`
	jwtCAYAML = `kind: cert_authority
metadata:
  name: me.localhost
  revision: a3f0310c-80fe-4c43-95d5-ba3f3cf9aedc
spec:
  active_keys:
    jwt:
    - private_key: LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JR0hBZ0VBTUJNR0J5cUdTTTQ5QWdFR0NDcUdTTTQ5QXdFSEJHMHdhd0lCQVFRZ3dFaDQ2empZTHltT0ZqNWIKSVZDNXBkMUxSbWU1clAzTmRVN3hKQ0plMDJ1aFJBTkNBQVRYQXA5MFRRTmowaDhCNHllR0dnNFJWQStsbVV4MwpRN3EzdFBLbEJaSVpiMWtXSHdSRmFXdUJmYUQ3Sis0MnUwSjRvVGxqYU5zUUg2Vm8wRXRLTkI4ZwotLS0tLUVORCBQUklWQVRFIEtFWS0tLS0tCg==
      public_key: LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUZrd0V3WUhLb1pJemowQ0FRWUlLb1pJemowREFRY0RRZ0FFMXdLZmRFMERZOUlmQWVNbmhob09FVlFQcFpsTQpkME82dDdUeXBRV1NHVzlaRmg4RVJXbHJnWDJnK3lmdU5ydENlS0U1WTJqYkVCK2xhTkJMU2pRZklBPT0KLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0tCg==
  additional_trusted_keys: {}
  cluster_name: me.localhost
  type: jwt
sub_kind: jwt
version: v2`
	openSSHCAYAML = `kind: cert_authority
metadata:
  name: me.localhost
  revision: 2c49b2de-7529-4e8f-9442-dec2357f2e93
spec:
  active_keys:
    ssh:
    - private_key: LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1DNENBUUF3QlFZREsyVndCQ0lFSUhoT0o3KzZlek4raTZ5dVVhVDBPT01rVm1NK0pIZm41Sm5ONzJFNmduWTgKLS0tLS1FTkQgUFJJVkFURSBLRVktLS0tLQo=
      public_key: c3NoLWVkMjU1MTkgQUFBQUMzTnphQzFsWkRJMU5URTVBQUFBSVBuQ0ZocHk3TUdBZTNBSDBjTDBkMmVUK20xNVF1dmptMUU2QXlubm1uSkYK
  additional_trusted_keys: {}
  cluster_name: me.localhost
  type: openssh
sub_kind: openssh
version: v2`
	samlCAYAML = `kind: cert_authority
metadata:
  name: me.localhost
  revision: 252e2c73-6805-48d4-8491-dde615ff8c47
spec:
  active_keys:
    tls:
    - cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURqVENDQW5XZ0F3SUJBZ0lSQUtDdHRGNGZ1V0ZTSzJaNm1mb3VPcmt3RFFZSktvWklodmNOQVFFTEJRQXcKWURFVk1CTUdBMVVFQ2hNTWJXVXViRzlqWVd4b2IzTjBNUlV3RXdZRFZRUURFd3h0WlM1c2IyTmhiR2h2YzNReApNREF1QmdOVkJBVVRKekl4TXpVM09EUXdORGszTXpFd056RTBORFkxTWpJMk9EVXlOelV5T0Rjek1qRTBOak0yCk1UQWVGdzB5TkRFeU16RXhOREkyTURaYUZ3MHpOREV5TWpreE5ESTJNRFphTUdBeEZUQVRCZ05WQkFvVERHMWwKTG14dlkyRnNhRzl6ZERFVk1CTUdBMVVFQXhNTWJXVXViRzlqWVd4b2IzTjBNVEF3TGdZRFZRUUZFeWN5TVRNMQpOemcwTURRNU56TXhNRGN4TkRRMk5USXlOamcxTWpjMU1qZzNNekl4TkRZek5qRXdnZ0VpTUEwR0NTcUdTSWIzCkRRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRREVBQTFtbG1jeU5uK3BiYzVtRkxVRWNFN3JSWWtxNlEydnNKdnYKd0F3dDRpbjhGSmoxdVJVenU1YUJqYUs5bERDN1cyd2JwSjI4SGZuZmtkb2ttOHN4T0xqTTh5YUlaSzBYNG9ZcwpTTDFhdFVPMUsrUkt5citJNGc0dzJBd05QUURsUE5xU0t3dlNzRXhsQzhSSXpUTllZYXVpQVUxaWdoVW5zZTUxCm5jMVRTbzJ0NStuemNoVEt2SU1qV0U5TUpmZ2lBRDRjVXlHVU5DQXV1dTRndWJEZVBFdStYZmVhUjNCcXJoNUwKaTRiWGdQanF3NnBLMTZzY1h5QlYwL1RzdGsxYXhpekh3M2h4SG5INDNFeUtsUVNneXBPNnNLTFFmR0VEVTZ6UgpjdlFIOHNOc0g0YmVNL2Zzb0RoeXFZQVZKbDVNT011UG0zTHBpdHJ1YktEOXA4S1ZBZ01CQUFHalFqQkFNQTRHCkExVWREd0VCL3dRRUF3SUJwakFQQmdOVkhSTUJBZjhFQlRBREFRSC9NQjBHQTFVZERnUVdCQlJqNytSM3pabEoKRlpPU05pM2xsOHhUclNQNVREQU5CZ2txaGtpRzl3MEJBUXNGQUFPQ0FRRUFwZmJHWXA5Q1hoNksvL1FKY3kyUQpqVjF0RDk1akZYcVVyZ0JrZ1g0cmplaEVqQjd0N3k1SnNXZlJIK25DL2UxaGRqL3dlZWMxenR1MFJNazUxRThlCmJCNWFaLzV1eVhMWXlMVGJWaVpZQzEzV0EyTlZpY200d08yWDQvK29NNzVQTzFwUmRvTm4weGRlcDFPa2o2VDcKQlA3WVM3SUhYRlMwUE00bUQxRDRsTHRIQmhqcjdJQnNlK2ZSRXBaZEJZMktqOTZ3NjZzT0c1RlNNMVY0NWRiNQoyY1lhTHFEcGR3aFVEY0xDZWVIVnYzaTB4UGk2TUJmWTFoVGd1MjRTMFpHZnhpR2F5Qk5FWEpKL1dtNk5kQkFUClhVckV2YkU3Z2tORWRkeXJPUW9sWDA5R3lYU0dNbDVXMTVtMHNVaythNk5RanlVQTVxZU1sTTNWN1Q1REVLcWgKV0E9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
      key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBeEFBTlpwWm5NalovcVczT1poUzFCSEJPNjBXSkt1a05yN0NiNzhBTUxlSXAvQlNZCjlia1ZNN3VXZ1kyaXZaUXd1MXRzRzZTZHZCMzUzNUhhSkp2TE1UaTR6UE1taUdTdEYrS0dMRWk5V3JWRHRTdmsKU3NxL2lPSU9NTmdNRFQwQTVUemFraXNMMHJCTVpRdkVTTTB6V0dHcm9nRk5Zb0lWSjdIdWRaM05VMHFOcmVmcAo4M0lVeXJ5REkxaFBUQ1g0SWdBK0hGTWhsRFFnTHJydUlMbXczanhMdmwzM21rZHdhcTRlUzR1RzE0RDQ2c09xClN0ZXJIRjhnVmRQMDdMWk5Xc1lzeDhONGNSNXgrTnhNaXBVRW9NcVR1ckNpMEh4aEExT3MwWEwwQi9MRGJCK0cKM2pQMzdLQTRjcW1BRlNaZVREakxqNXR5NllyYTdteWcvYWZDbFFJREFRQUJBb0lCQUZtSU1KYnBJM0REaG1OdAozbmV4QTlOb1BoU282ZlNwQ3ZCemUzZjBRVndBVU85dXRVU2g3RFo2ZlZEbTB5MUljVTVVZjdqTTVLVFhDSnFBCjlLWCthTDR1Uy9TTEtkSHFNMHVTMVhtTExMd3Z5eU1LVHJsL2ppaklJblZiYTMzc25Pa2FlRG1HNGxxMjM5N1UKbGpBdlZFSU9NNm5JY0lJTUsvKzYvdFBKWnM2aGxXZXAzcGRuOWMwUXBUWGExbGpiZ1FVMHVNN2NwanRLRE4rcwpYRGRzb2N3VGpUV2MwNGY0aDBtS3NkdmlxeG80bVBSK0tyV3Brb1pFdFhiUGQ4THg5OVNkQjJ0aEs2enBiNEErCmZKOFlXYjhZTWp3SGpXRW5OYVpiM2FocGVuRkVyVWZFb1hlRGc4ajltak1VSGFSUURIK3ZGN0RINlI3U3lRb1QKOWo2dFBXRUNnWUVBemptNCtKamswbEd3SmVDMzAyN01Fd2lQMlNwcG0yRlNLY1MzVnM0cmE3TGhFR1pSZEpBcQpYSDNpdEVjZWViZTg4aU1RcDJXam1DMktEN1ZLOTBTN2pVUUpLbEw2eTM5WDh0RGlJRHQ4Y3l2OVd6TTJEK0p2Cm0vc3BFc1A4aURJTEVYZDJYUk1JNHhja2tXRml0TkE0SDNnODNtYWRPVWkzOEsyN0xyR3Q0b2tDZ1lFQTgwNkgKbWJOY1hKSTRML09FdTdHd2hVSk1uOGdLaWpvWStMdmZvQlN5Ym9SdzFIWkNvYnRZM2NLd0xNSnAvNkdQVG1lbwpucWlXNWs0YjJLVlpuczl0elNSRWhZa1NNTWRyZllhTis3bGZKTEozS3BhenNXMGlWbk5HaFlidzZVSnY0TmpnCjMzYnQvemh4bkp5ckhzS3B0VndiWm9lWmN2RDFkcVB5cm5kOVRLMENnWUVBdDkvYnZ6eUQrY3NBSmlYQmdmR3UKWCtJb2NGZFNwa29WK2t2OXRKWkxQTkhYdnNtY0l6UlBzUHhGWUx4d3ZkSkgxQlhUeVkza1dkRnc0aVNoWE91WgoxcEV0SXVHdDREZ0E4TzJ5VVU3NDNiQUJUSW5TMEVMemhMNWlsdXJNaFpzcEp6Kys5Nm43S0kvLytPZytIRDN6CmJJdkdxZjRRZlgwTEZMdXl4Q1dFaHhFQ2dZQVk2eG9JSzg1eHpLZmtnVlEreE53SFNkci9Ja1d5RW5Fc1NGR0cKMjVmS3FkWEViTGcyU0RHNXhJNjJodExFVTQrUndCd000OGRRbnY5TEdPUXMxNkd2T04rcnJYWW5lTVVSZmc1YwprWWVsQW9JaDRuMVUxcENGdWhpbTVFTVlJSzNFb1hHbWNVKytxOUUyOFBTMW1jbzN3TTh0bVFXbU4vZHJ4eTY3Cm41RTlvUUtCZ0R4VmptWGpCd1hkY3RhQm8ra3JHT1JWV2hJUzVWUFQrVkgwQ1F2MTUxcCtvT2EyMldQNSttbSsKUjJrcEhWNXRVTmUyUE9VZU5oMVE5U3JKQ1NLWmQ2K0JsT0U4bjQvc0kyNDJvbmRnelhrL21LRnc4NjBMQmdUbApkWTRvTC9mUU1mQ242aU4vMXBGZmFhWDZSV1lVMUplSTduK3VFZ0hDTFE4Z2x3eU1JcFdmCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
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

// TestInitWithAutoUpdateResources verifies that auth init support bootstrapping and apply
// `AutoUpdateConfig` and `AutoUpdateVersion` resources as well as unmarshalling them from
// yaml configuration.
func TestInitWithAutoUpdateResources(t *testing.T) {
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
	resources := []types.Resource{
		resourceFromYAML(t, autoUpdateConfigYAML),
		resourceFromYAML(t, autoUpdateVersionYAML),
	}

	for _, test := range []struct {
		name string
		fn   func(cfg *InitConfig)
	}{
		{name: "bootstrap", fn: func(cfg *InitConfig) { cfg.BootstrapResources = resources }},
		{name: "apply", fn: func(cfg *InitConfig) { cfg.ApplyOnStartupResources = resources }},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := setupConfig(t)
			test.fn(&cfg)
			auth, err := Init(ctx, cfg)
			require.NoError(t, err)

			config, err := auth.GetAutoUpdateConfig(ctx)
			assert.NoError(t, err)
			assert.Equal(t, "enabled", config.GetSpec().GetTools().GetMode())

			version, err := auth.GetAutoUpdateVersion(ctx)
			assert.NoError(t, err)
			assert.Equal(t, "1.2.3", version.GetSpec().GetTools().GetTargetVersion())
		})
	}
}
