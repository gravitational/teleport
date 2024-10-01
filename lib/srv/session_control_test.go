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

package srv

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

type mockLockEnforcer struct {
	lockInForceErr error
}

func (m mockLockEnforcer) CheckLockInForce(constants.LockingMode, ...types.LockTarget) error {
	return m.lockInForceErr
}

type mockAccessPoint struct {
	AccessPoint

	authPreference types.AuthPreference
	clusterName    types.ClusterName
	netConfig      types.ClusterNetworkingConfig
}

func (m mockAccessPoint) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return m.authPreference, nil
}

func (m mockAccessPoint) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	return m.clusterName, nil
}

func (m mockAccessPoint) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	return m.netConfig, nil
}

type mockSemaphores struct {
	types.Semaphores

	lease      *types.SemaphoreLease
	acquireErr error
}

func (m mockSemaphores) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	return m.lease, m.acquireErr
}

func (m mockSemaphores) CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	return nil
}

type mockAccessChecker struct {
	services.AccessChecker

	lockMode       constants.LockingMode
	maxConnections int64
	keyPolicy      keys.PrivateKeyPolicy
	roleNames      []string
}

func (m mockAccessChecker) LockingMode(defaultMode constants.LockingMode) constants.LockingMode {
	return m.lockMode
}

func (m mockAccessChecker) MaxConnections() int64 {
	return m.maxConnections
}

func (m mockAccessChecker) PrivateKeyPolicy(defaultPolicy keys.PrivateKeyPolicy) (keys.PrivateKeyPolicy, error) {
	return m.keyPolicy, nil
}

func (m mockAccessChecker) RoleNames() []string {
	return m.roleNames
}

func TestSessionController_AcquireSessionContext(t *testing.T) {
	clock := clockwork.NewFakeClock()
	emitter := &eventstest.MockRecorderEmitter{}

	minimalCfg := SessionControllerConfig{
		Semaphores: mockSemaphores{},
		AccessPoint: mockAccessPoint{
			authPreference: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{},
			},
			clusterName: &types.ClusterNameV2{
				Spec: types.ClusterNameSpecV2{
					ClusterName: "llama",
				},
			},
		},
		LockEnforcer: mockLockEnforcer{},
		Emitter:      emitter,
		Component:    teleport.ComponentNode,
		ServerID:     "1234",
	}

	minimalIdentity := IdentityContext{
		TeleportUser: "alpaca",
		Login:        "alpaca",
		Certificate: &ssh.Certificate{
			KeyId: "alpaca",
		},
		AccessChecker: &mockAccessChecker{
			keyPolicy: keys.PrivateKeyPolicyNone,
		},
	}

	cfgWithDeviceMode := func(mode string) SessionControllerConfig {
		cfg := minimalCfg
		authPref, _ := cfg.AccessPoint.GetAuthPreference(context.Background())
		authPref.(*types.AuthPreferenceV2).Spec.DeviceTrust = &types.DeviceTrust{
			Mode: mode,
		}
		return cfg
	}
	identityWithDeviceExtensions := func() IdentityContext {
		idCtx := minimalIdentity
		idCtx.Certificate = &ssh.Certificate{
			KeyId: "alpaca",
			Permissions: ssh.Permissions{
				Extensions: map[string]string{
					teleport.CertExtensionDeviceID:           "deviceid1",
					teleport.CertExtensionDeviceAssetTag:     "assettag1",
					teleport.CertExtensionDeviceCredentialID: "credentialid1",
				},
			},
		}
		return idCtx
	}
	assertTrustedDeviceRequired := func(t *testing.T, _ context.Context, err error, _ *eventstest.MockRecorderEmitter) {
		assert.ErrorContains(t, err, "device", "AcquireSessionContext returned an unexpected error")
		assert.True(t, trace.IsAccessDenied(err), "AcquireSessionContext returned an error other than trace.AccessDeniedError: %T", err)
	}

	cases := []struct {
		name      string
		buildType string // defaults to modules.BuildOSS
		cfg       SessionControllerConfig
		identity  IdentityContext
		assertion func(t *testing.T, ctx context.Context, err error, emitter *eventstest.MockRecorderEmitter)
	}{
		{
			name: "proxy: access allowed",
			cfg: SessionControllerConfig{
				Semaphores: mockSemaphores{},
				AccessPoint: mockAccessPoint{
					netConfig: &types.ClusterNetworkingConfigV2{},
					authPreference: &types.AuthPreferenceV2{
						Spec: types.AuthPreferenceSpecV2{
							LockingMode: constants.LockingModeStrict,
						},
					},
					clusterName: &types.ClusterNameV2{Spec: types.ClusterNameSpecV2{ClusterName: "llama"}},
				},
				LockEnforcer: mockLockEnforcer{},
				Emitter:      emitter,
				Component:    teleport.ComponentProxy,
				ServerID:     "1234",
			},
			identity: IdentityContext{
				TeleportUser: "alpaca",
				Login:        "alpaca",
				Certificate: &ssh.Certificate{
					Permissions: ssh.Permissions{
						Extensions: map[string]string{
							teleport.CertExtensionPrivateKeyPolicy: string(keys.PrivateKeyPolicyNone),
						},
					},
				},
				AccessChecker: mockAccessChecker{
					keyPolicy:      keys.PrivateKeyPolicyNone,
					maxConnections: 1,
				},
			},
			assertion: func(t *testing.T, ctx context.Context, err error, emitter *eventstest.MockRecorderEmitter) {
				require.NoError(t, err)
				require.NotNil(t, ctx)
				require.Empty(t, emitter.Events())
			},
		},
		{
			name: "node: access allowed",
			cfg: SessionControllerConfig{
				Clock: clock,
				Semaphores: mockSemaphores{
					lease: &types.SemaphoreLease{
						SemaphoreKind: types.SemaphoreKindConnection,
						SemaphoreName: "test",
						LeaseID:       "1",
						Expires:       clock.Now().Add(time.Minute),
					},
				},
				AccessPoint: mockAccessPoint{
					netConfig: &types.ClusterNetworkingConfigV2{
						Spec: types.ClusterNetworkingConfigSpecV2{
							SessionControlTimeout: types.NewDuration(time.Minute),
						},
					},
					authPreference: &types.AuthPreferenceV2{
						Spec: types.AuthPreferenceSpecV2{
							LockingMode: constants.LockingModeStrict,
						},
					},
					clusterName: &types.ClusterNameV2{Spec: types.ClusterNameSpecV2{ClusterName: "llama"}},
				},
				LockEnforcer: mockLockEnforcer{},
				Emitter:      emitter,
				Component:    teleport.ComponentNode,
				ServerID:     "1234",
			},
			identity: IdentityContext{
				TeleportUser: "alpaca",
				Login:        "alpaca",
				Certificate: &ssh.Certificate{
					Permissions: ssh.Permissions{
						Extensions: map[string]string{
							teleport.CertExtensionPrivateKeyPolicy: string(keys.PrivateKeyPolicyNone),
						},
					},
				},
				AccessChecker: mockAccessChecker{
					keyPolicy:      keys.PrivateKeyPolicyNone,
					maxConnections: 1,
				},
			},
			assertion: func(t *testing.T, ctx context.Context, err error, emitter *eventstest.MockRecorderEmitter) {
				require.NoError(t, err)
				require.NotNil(t, ctx)
				require.Empty(t, emitter.Events())
			},
		},
		{
			name: "session rejected due to lock",
			cfg: SessionControllerConfig{
				Clock:      clock,
				Semaphores: mockSemaphores{},
				AccessPoint: mockAccessPoint{
					authPreference: &types.AuthPreferenceV2{
						Spec: types.AuthPreferenceSpecV2{
							LockingMode: constants.LockingModeStrict,
						},
					},
					clusterName: &types.ClusterNameV2{Spec: types.ClusterNameSpecV2{ClusterName: "llama"}},
				},
				LockEnforcer: mockLockEnforcer{
					lockInForceErr: trace.AccessDenied("lock in force"),
				},
				Emitter:   emitter,
				Component: teleport.ComponentNode,
				ServerID:  "1234",
			},
			identity: IdentityContext{
				TeleportUser: "alpaca",
				Login:        "alpaca",
				Certificate: &ssh.Certificate{
					Permissions: ssh.Permissions{
						Extensions: map[string]string{
							teleport.CertExtensionPrivateKeyPolicy: string(keys.PrivateKeyPolicyNone),
						},
					},
				},
				AccessChecker: mockAccessChecker{
					keyPolicy:      keys.PrivateKeyPolicyNone,
					maxConnections: 1,
				},
			},
			assertion: func(t *testing.T, ctx context.Context, err error, emitter *eventstest.MockRecorderEmitter) {
				require.ErrorIs(t, err, trace.AccessDenied("lock in force"))
				require.NotNil(t, ctx)
				require.Len(t, emitter.Events(), 1)

				evt, ok := emitter.Events()[0].(*apievents.SessionReject)
				require.True(t, ok)
				require.Equal(t, events.SessionRejectedEvent, evt.Metadata.Type)
				require.Equal(t, events.SessionRejectedCode, evt.Metadata.Code)
				require.Equal(t, events.EventProtocolSSH, evt.ConnectionMetadata.Protocol)
				require.Equal(t, "lock in force", evt.Reason)
			},
		},
		{
			name: "session rejected due to private key policy",
			cfg: SessionControllerConfig{
				Clock:      clock,
				Semaphores: mockSemaphores{},
				AccessPoint: mockAccessPoint{
					authPreference: &types.AuthPreferenceV2{
						Spec: types.AuthPreferenceSpecV2{
							LockingMode: constants.LockingModeStrict,
						},
					},
					clusterName: &types.ClusterNameV2{Spec: types.ClusterNameSpecV2{ClusterName: "llama"}},
				},
				LockEnforcer: mockLockEnforcer{},
				Emitter:      emitter,
				Component:    teleport.ComponentNode,
				ServerID:     "1234",
			},
			identity: IdentityContext{
				TeleportUser: "alpaca",
				Login:        "alpaca",
				Certificate: &ssh.Certificate{
					Permissions: ssh.Permissions{
						Extensions: map[string]string{
							teleport.CertExtensionPrivateKeyPolicy: string(keys.PrivateKeyPolicyNone),
						},
					},
				},
				AccessChecker: mockAccessChecker{
					keyPolicy:      keys.PrivateKeyPolicyHardwareKey,
					maxConnections: 1,
				},
			},
			assertion: func(t *testing.T, ctx context.Context, err error, emitter *eventstest.MockRecorderEmitter) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
				require.NotNil(t, ctx)
				require.Empty(t, emitter.Events())
			},
		},
		{
			name: "session rejected due to connection limit",
			cfg: SessionControllerConfig{
				Clock: clock,
				Semaphores: mockSemaphores{
					acquireErr: trace.LimitExceeded(teleport.MaxLeases),
				},
				AccessPoint: mockAccessPoint{
					authPreference: &types.AuthPreferenceV2{
						Spec: types.AuthPreferenceSpecV2{
							LockingMode: constants.LockingModeStrict,
						},
					},
					clusterName: &types.ClusterNameV2{Spec: types.ClusterNameSpecV2{ClusterName: "llama"}},
					netConfig: &types.ClusterNetworkingConfigV2{
						Spec: types.ClusterNetworkingConfigSpecV2{
							SessionControlTimeout: types.NewDuration(time.Minute),
						},
					},
				},
				LockEnforcer: mockLockEnforcer{},
				Emitter:      emitter,
				Component:    teleport.ComponentNode,
				ServerID:     "1234",
			},
			identity: IdentityContext{
				TeleportUser: "alpaca",
				Login:        "alpaca",
				Certificate: &ssh.Certificate{
					Permissions: ssh.Permissions{
						Extensions: map[string]string{
							teleport.CertExtensionPrivateKeyPolicy: string(keys.PrivateKeyPolicyNone),
						},
					},
				},
				AccessChecker: mockAccessChecker{
					keyPolicy:      keys.PrivateKeyPolicyNone,
					maxConnections: 1,
				},
			},
			assertion: func(t *testing.T, ctx context.Context, err error, emitter *eventstest.MockRecorderEmitter) {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
				require.NotNil(t, ctx)
				require.Len(t, emitter.Events(), 1)

				evt, ok := emitter.Events()[0].(*apievents.SessionReject)
				require.True(t, ok)
				require.Equal(t, events.SessionRejectedEvent, evt.Metadata.Type)
				require.Equal(t, events.SessionRejectedCode, evt.Metadata.Code)
				require.Equal(t, events.EventProtocolSSH, evt.ConnectionMetadata.Protocol)
				require.Equal(t, events.SessionRejectedEvent, evt.Reason)
				require.Equal(t, int64(1), evt.Maximum)
			},
		},
		{
			name: "no connection limits prevent acquiring semaphore lock",
			cfg: SessionControllerConfig{
				Clock: clock,
				Semaphores: mockSemaphores{
					acquireErr: trace.LimitExceeded(teleport.MaxLeases),
				},
				AccessPoint: mockAccessPoint{
					authPreference: &types.AuthPreferenceV2{
						Spec: types.AuthPreferenceSpecV2{
							LockingMode: constants.LockingModeStrict,
						},
					},
					clusterName: &types.ClusterNameV2{Spec: types.ClusterNameSpecV2{ClusterName: "llama"}},
					netConfig: &types.ClusterNetworkingConfigV2{
						Spec: types.ClusterNetworkingConfigSpecV2{
							SessionControlTimeout: types.NewDuration(time.Minute),
						},
					},
				},
				LockEnforcer: mockLockEnforcer{},
				Emitter:      emitter,
				Component:    teleport.ComponentNode,
				ServerID:     "1234",
			},
			identity: IdentityContext{
				TeleportUser: "alpaca",
				Login:        "alpaca",
				Certificate: &ssh.Certificate{
					Permissions: ssh.Permissions{
						Extensions: map[string]string{
							teleport.CertExtensionPrivateKeyPolicy: string(keys.PrivateKeyPolicyNone),
						},
					},
				},
				AccessChecker: mockAccessChecker{
					keyPolicy:      keys.PrivateKeyPolicyNone,
					maxConnections: 0,
				},
			},
			assertion: func(t *testing.T, ctx context.Context, err error, emitter *eventstest.MockRecorderEmitter) {
				require.NoError(t, err)
				require.NotNil(t, ctx)
				require.Empty(t, emitter.Events(), 0)
			},
		},
		{
			name:      "device extensions enforced for OSS",
			cfg:       cfgWithDeviceMode(constants.DeviceTrustModeRequired),
			identity:  minimalIdentity,
			assertion: assertTrustedDeviceRequired,
		},
		{
			name:      "device extensions enforced for Enterprise",
			buildType: modules.BuildEnterprise,
			cfg:       cfgWithDeviceMode(constants.DeviceTrustModeRequired),
			identity:  minimalIdentity,
			assertion: assertTrustedDeviceRequired,
		},
		{
			name:      "device extensions valid for Enterprise",
			buildType: modules.BuildEnterprise,
			cfg:       cfgWithDeviceMode(constants.DeviceTrustModeRequired),
			identity:  identityWithDeviceExtensions(),
			assertion: func(t *testing.T, _ context.Context, err error, _ *eventstest.MockRecorderEmitter) {
				assert.NoError(t, err, "AcquireSessionContext returned an unexpected error")
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			buildType := tt.buildType
			if buildType == "" {
				buildType = modules.BuildOSS
			}
			modules.SetTestModules(t, &modules.TestModules{
				TestBuildType: buildType,
			})

			emitter.Reset()
			ctrl, err := NewSessionController(tt.cfg)
			require.NoError(t, err, "NewSessionController failed")

			ctx, err := ctrl.AcquireSessionContext(context.Background(), tt.identity, "127.0.0.1:1", "127.0.0.1:2")
			tt.assertion(t, ctx, err, emitter)
		})
	}
}
