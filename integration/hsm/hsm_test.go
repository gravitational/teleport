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

package hsm

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	// Enable HSM feature.
	// This is safe to do here, as all tests in this package require HSM to be
	// enabled.
	modules.SetModules(&modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.HSM: {Enabled: true},
			},
		},
	})

	os.Exit(m.Run())
}

func newHSMAuthConfig(t *testing.T, storageConfig *backend.Config, log *slog.Logger, clock clockwork.Clock) *servicecfg.Config {
	config := newAuthConfig(t, log, clock)
	config.Auth.StorageConfig = *storageConfig
	config.Auth.KeyStore = keystore.HSMTestConfig(t)
	authPref, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		SignatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
	})
	require.NoError(t, err)
	config.Auth.Preference = authPref
	return config
}

func liteBackendConfig(t *testing.T) *backend.Config {
	return &backend.Config{
		Type: lite.GetName(),
		Params: backend.Params{
			"path": t.TempDir(),
		},
	}
}

// Tests a single CA rotation with a single HSM auth server
func TestHSMRotation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	log := utils.NewSlogLoggerForTests().With(teleport.ComponentKey, "TestHSMRotation")

	log.DebugContext(ctx, "starting auth server")
	authConfig := newHSMAuthConfig(t, liteBackendConfig(t), log, clockwork.NewRealClock())
	auth1, err := newTeleportService(ctx, authConfig, "auth1")
	require.NoError(t, err)
	allServices := teleportServices{auth1}

	t.Cleanup(func() {
		require.NoError(t, auth1.process.GetAuthServer().GetKeyStore().DeleteUnusedKeys(ctx, nil))
	})

	// start a proxy to make sure it can get creds at each stage of rotation
	log.DebugContext(ctx, "starting proxy")
	proxy, err := newTeleportService(ctx, newProxyConfig(t, auth1.authAddr(t), log, clockwork.NewRealClock()), "proxy")
	require.NoError(t, err)
	allServices = append(allServices, proxy)

	log.DebugContext(ctx, "sending rotation request init")
	require.NoError(t, allServices.waitingForNewEvent(ctx, service.TeleportPhaseChangeEvent, func() error {
		return trace.Wrap(auth1.process.GetAuthServer().RotateCertAuthority(ctx, types.RotateRequest{
			Type:        types.HostCA,
			TargetPhase: types.RotationPhaseInit,
			Mode:        types.RotationModeManual,
		}))
	}))

	log.DebugContext(ctx, "sending rotation request update_clients")
	require.NoError(t, allServices.waitingForNewEvent(ctx, service.TeleportCredentialsUpdatedEvent, func() error {
		return trace.Wrap(auth1.process.GetAuthServer().RotateCertAuthority(ctx, types.RotateRequest{
			Type:        types.HostCA,
			TargetPhase: types.RotationPhaseUpdateClients,
			Mode:        types.RotationModeManual,
		}))
	}))

	log.DebugContext(ctx, "sending rotation request update_servers")
	require.NoError(t, allServices.waitingForNewEvent(ctx, service.TeleportCredentialsUpdatedEvent, func() error {
		return trace.Wrap(auth1.process.GetAuthServer().RotateCertAuthority(ctx, types.RotateRequest{
			Type:        types.HostCA,
			TargetPhase: types.RotationPhaseUpdateServers,
			Mode:        types.RotationModeManual,
		}))
	}))

	log.DebugContext(ctx, "sending rotation request standby")
	require.NoError(t, allServices.waitingForNewEvent(ctx, service.TeleportCredentialsUpdatedEvent, func() error {
		return trace.Wrap(auth1.process.GetAuthServer().RotateCertAuthority(ctx, types.RotateRequest{
			Type:        types.HostCA,
			TargetPhase: types.RotationPhaseStandby,
			Mode:        types.RotationModeManual,
		}))
	}))
}

func getAdminClient(authDataDir string, authAddr string) (*authclient.Client, error) {
	identity, err := storage.ReadLocalIdentity(
		filepath.Join(authDataDir, teleport.ComponentProcess),
		state.IdentityID{Role: types.RoleAdmin})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig, err := identity.TLSConfig(nil /*cipherSuites*/)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := authclient.NewClient(client.Config{
		Addrs: []string{authAddr},
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		CircuitBreakerConfig: breaker.NoopBreakerConfig(),
	})
	return clt, trace.Wrap(err)
}

func testAdminClient(t *testing.T, authDataDir string, authAddr string) {
	f := func() error {
		clt, err := getAdminClient(authDataDir, authAddr)
		if err != nil {
			return err
		}
		defer clt.Close()
		_, err = clt.GetClusterName(context.TODO())
		return err
	}
	// We might be hitting a load balancer in front of two auths, running
	// the check twice gives us a better chance of testing both
	//
	// Eventually(WithT) always waits at the beginning, but we have a good
	// chance of succeeding immediately, and we end up calling this quite a
	// few times, so this saves us a lot of waiting
	//
	// staticcheck can't figure out that functions might have side effects so
	// this can't just be "f() == nil && f() == nil"
	if err1, err2 := f(), f(); err1 == nil && err2 == nil {
		return
	}
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.NoError(t, f())
		assert.NoError(t, f())
	}, 10*time.Second, 250*time.Millisecond, "admin client failed test call to GetClusterName")
}

// Tests multiple CA rotations and rollbacks with 2 HSM auth servers in an HA configuration
func TestHSMDualAuthRotation(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SKIP_VERSION_UPGRADE_CHECK", "1")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	log := utils.NewSlogLoggerForTests().With(teleport.ComponentKey, "TestHSMDualAuthRotation")
	storageConfig := liteBackendConfig(t)

	// start a cluster with 1 auth server
	log.DebugContext(ctx, "Starting auth server 1")
	auth1Config := newHSMAuthConfig(t, storageConfig, log, clockwork.NewRealClock())
	auth1, err := newTeleportService(ctx, auth1Config, "auth1")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, auth1.process.GetAuthServer().GetKeyStore().DeleteUnusedKeys(ctx, nil),
			"failed to delete hsm keys during test cleanup")
	})
	authServices := teleportServices{auth1}

	log.DebugContext(ctx, "Starting load balancer")
	lb, err := utils.NewLoadBalancer(
		ctx,
		*utils.MustParseAddr(net.JoinHostPort("localhost", "0")),
		auth1.authAddr(t),
	)
	require.NoError(t, err)
	require.NoError(t, lb.Listen())
	go lb.Serve()
	t.Cleanup(func() { require.NoError(t, lb.Close()) })

	// add a new auth server
	log.DebugContext(ctx, "Starting auth server 2")
	auth2Config := newHSMAuthConfig(t, storageConfig, log, clockwork.NewRealClock())
	auth2, err := newTeleportService(ctx, auth2Config, "auth2")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, auth2.process.GetAuthServer().GetKeyStore().DeleteUnusedKeys(ctx, nil))
	})
	authServices = append(authServices, auth2)

	testAuth2Client := func(t *testing.T) {
		testAdminClient(t, auth2Config.DataDir, auth2.authAddrString(t))
	}
	testAuth2Client(t)

	verifyPhaseChangeAndAdditionalKeys := func(fn func() error) error {
		if err := authServices.waitingForNewEvent(ctx, service.TeleportPhaseChangeEvent, fn); err != nil {
			return err
		}
		if err := authServices.waitForLocalAdditionalKeys(ctx); err != nil {
			return err
		}
		return nil
	}
	verifyCredentialsUpdated := func(fn func() error) error {
		return authServices.waitingForNewEvent(ctx, service.TeleportCredentialsUpdatedEvent, fn)
	}

	stages := []struct {
		targetPhase string
		verify      func(func() error) error
	}{
		{
			targetPhase: types.RotationPhaseInit,
			verify:      verifyPhaseChangeAndAdditionalKeys,
		},
		{
			targetPhase: types.RotationPhaseUpdateClients,
			verify:      verifyCredentialsUpdated,
		},
		{
			targetPhase: types.RotationPhaseUpdateServers,
			verify:      verifyCredentialsUpdated,
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify:      verifyCredentialsUpdated,
		},
	}

	// do a full rotation
	for _, stage := range stages {
		log.DebugContext(ctx, "Sending rotate request", "phase", stage.targetPhase)
		require.NoError(t, stage.verify(func() error {
			return auth1.process.GetAuthServer().RotateCertAuthority(ctx, types.RotateRequest{
				Type:        types.HostCA,
				TargetPhase: stage.targetPhase,
				Mode:        types.RotationModeManual,
			})
		}))
		testAuth2Client(t)
	}

	// Safe to send traffic to new auth server now that a full rotation has been completed.
	lb.AddBackend(auth2.authAddr(t))

	testLoadBalancedClient := func(t *testing.T) {
		testAdminClient(t, auth2Config.DataDir, lb.Addr().String())
	}
	testLoadBalancedClient(t)

	// Do another full rotation from the new auth server
	for _, stage := range stages {
		log.DebugContext(ctx, "Sending rotate request", "phase", stage.targetPhase)
		require.NoError(t, stage.verify(func() error {
			return auth2.process.GetAuthServer().RotateCertAuthority(ctx, types.RotateRequest{
				Type:        types.HostCA,
				TargetPhase: stage.targetPhase,
				Mode:        types.RotationModeManual,
			})
		}))
		testAuth2Client(t)
	}

	// test rollbacks
	stages = []struct {
		targetPhase string
		verify      func(func() error) error
	}{
		{
			targetPhase: types.RotationPhaseInit,
			verify:      verifyPhaseChangeAndAdditionalKeys,
		},
		{
			targetPhase: types.RotationPhaseRollback,
			verify:      verifyCredentialsUpdated,
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify:      verifyCredentialsUpdated,
		},
		{
			targetPhase: types.RotationPhaseInit,
			verify:      verifyPhaseChangeAndAdditionalKeys,
		},
		{
			targetPhase: types.RotationPhaseUpdateClients,
			verify:      verifyCredentialsUpdated,
		},
		{
			targetPhase: types.RotationPhaseRollback,
			verify:      verifyCredentialsUpdated,
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify:      verifyCredentialsUpdated,
		},
		{
			targetPhase: types.RotationPhaseInit,
			verify:      verifyPhaseChangeAndAdditionalKeys,
		},
		{
			targetPhase: types.RotationPhaseUpdateClients,
			verify:      verifyCredentialsUpdated,
		},
		{
			targetPhase: types.RotationPhaseUpdateServers,
			verify:      verifyCredentialsUpdated,
		},
		{
			targetPhase: types.RotationPhaseRollback,
			verify:      verifyCredentialsUpdated,
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify:      verifyCredentialsUpdated,
		},
	}
	for _, stage := range stages {
		log.DebugContext(ctx, "Sending rotate request", "phase", stage.targetPhase)

		require.NoError(t, stage.verify(func() error {
			return auth1.process.GetAuthServer().RotateCertAuthority(ctx, types.RotateRequest{
				Type:        types.HostCA,
				TargetPhase: stage.targetPhase,
				Mode:        types.RotationModeManual,
			})
		}))
		testLoadBalancedClient(t)
	}
}

// Tests a dual-auth server migration from raw keys to HSM keys
func TestHSMMigrate(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SKIP_VERSION_UPGRADE_CHECK", "1")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	log := utils.NewSlogLoggerForTests().With(teleport.ComponentKey, "TestHSMMigrate")
	storageConfig := liteBackendConfig(t)

	// start a dual auth non-hsm cluster
	log.DebugContext(ctx, "Starting auth server 1")
	auth1Config := newHSMAuthConfig(t, storageConfig, log, clockwork.NewRealClock())
	auth1Config.Auth.KeyStore = servicecfg.KeystoreConfig{}
	auth2Config := newHSMAuthConfig(t, storageConfig, log, clockwork.NewRealClock())
	auth2Config.Auth.KeyStore = servicecfg.KeystoreConfig{}
	auth1, err := newTeleportService(ctx, auth1Config, "auth1")
	require.NoError(t, err)
	auth2, err := newTeleportService(ctx, auth2Config, "auth2")
	require.NoError(t, err)

	// Replace configured addresses with port set to 0 with the actual port
	// number so they are stable across hard restarts.
	auth1Config.Auth.ListenAddr = auth1.authAddr(t)
	auth2Config.Auth.ListenAddr = auth2.authAddr(t)

	log.DebugContext(ctx, "Starting load balancer")
	lb, err := utils.NewLoadBalancer(
		ctx,
		*utils.MustParseAddr(net.JoinHostPort("localhost", "0")),
		auth1.authAddr(t),
		auth2.authAddr(t),
	)
	require.NoError(t, err)
	require.NoError(t, lb.Listen())
	go lb.Serve()
	t.Cleanup(func() { require.NoError(t, lb.Close()) })

	testClient := func(t *testing.T) {
		testAdminClient(t, auth1Config.DataDir, lb.Addr().String())
	}
	testClient(t)

	// Phase 1: migrate auth1 to HSM
	auth1.process.Close()
	require.NoError(t, auth1.waitForShutdown(ctx))
	auth1Config.Auth.KeyStore = keystore.HSMTestConfig(t)
	auth1, err = newTeleportService(ctx, auth1Config, "auth1")
	require.NoError(t, err)

	testClient(t)

	// Make sure a cluster alert is created.
	alerts, err := auth1.process.GetAuthServer().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{})
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	alert := alerts[0]
	assert.Equal(t, types.AlertSeverity_MEDIUM, alert.Spec.Severity)
	assert.Contains(t, alert.Spec.Message, "configured to use PKCS#11 HSM keys")
	assert.Contains(t, alert.Spec.Message, "the following CAs do not contain any keys of that type:")
	assert.Contains(t, alert.Spec.Message, "host")

	authServices := teleportServices{auth1, auth2}

	verifyPhaseChangeAndAdditionalKeys := func(fn func() error) error {
		if err := authServices.waitingForNewEvent(ctx, service.TeleportPhaseChangeEvent, fn); err != nil {
			return err
		}
		if err := authServices.waitForLocalAdditionalKeys(ctx); err != nil {
			return err
		}
		return nil
	}
	verifyCredentialsUpdated := func(fn func() error) error {
		return authServices.waitingForNewEvent(ctx, service.TeleportCredentialsUpdatedEvent, fn)
	}
	stages := []struct {
		targetPhase string
		verify      func(func() error) error
	}{
		{
			targetPhase: types.RotationPhaseInit,
			verify:      verifyPhaseChangeAndAdditionalKeys,
		},
		{
			targetPhase: types.RotationPhaseUpdateClients,
			verify:      verifyCredentialsUpdated,
		},
		{
			targetPhase: types.RotationPhaseUpdateServers,
			verify:      verifyCredentialsUpdated,
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify:      verifyCredentialsUpdated,
		},
	}

	// Do a full rotation to get HSM keys for auth1 into the CA.
	for _, stage := range stages {
		log.DebugContext(ctx, "Sending rotate request", "phase", stage.targetPhase)
		require.NoError(t, stage.verify(func() error {
			return auth1.process.GetAuthServer().RotateCertAuthority(ctx, types.RotateRequest{
				Type:        types.HostCA,
				TargetPhase: stage.targetPhase,
				Mode:        types.RotationModeManual,
			})
		}))
		testClient(t)
	}

	// Make sure the cluster alert no longer mentions the host CA.
	require.NoError(t, auth1.process.GetAuthServer().AutoRotateCertAuthorities(ctx))
	alerts, err = auth1.process.GetAuthServer().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{})
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	alert = alerts[0]
	assert.NotContains(t, alert.Spec.Message, "host")

	// Phase 2: migrate auth2 to HSM
	auth2.process.Close()
	require.NoError(t, auth2.waitForShutdown(ctx))
	auth2Config.Auth.KeyStore = keystore.HSMTestConfig(t)
	auth2, err = newTeleportService(ctx, auth2Config, "auth2")
	require.NoError(t, err)
	authServices = teleportServices{auth1, auth2}

	testClient(t)

	// There should now be 2 cluster alerts (one for each auth using HSM).
	alerts, err = auth1.process.GetAuthServer().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{})
	require.NoError(t, err)
	assert.Len(t, alerts, 2)

	// Do another full rotation to get HSM keys for auth2 into the CA.
	for _, stage := range stages {
		log.DebugContext(ctx, "Sending rotate request", "phase", stage.targetPhase)
		require.NoError(t, stage.verify(func() error {
			return auth2.process.GetAuthServer().RotateCertAuthority(ctx, types.RotateRequest{
				Type:        types.HostCA,
				TargetPhase: stage.targetPhase,
				Mode:        types.RotationModeManual,
			})
		}))
		testClient(t)
	}
}

// TestHSMRevert tests a single-auth server migration from HSM keys back to
// software keys.
func TestHSMRevert(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	log := utils.NewSlogLoggerForTests().With(teleport.ComponentKey, "TestHSMRevert")

	log.DebugContext(ctx, "starting auth server")
	auth1Config := newHSMAuthConfig(t, liteBackendConfig(t), log, clock)
	auth1, err := newTeleportService(ctx, auth1Config, "auth1")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, auth1.process.GetAuthServer().GetKeyStore().DeleteUnusedKeys(ctx, nil))
	})

	// Switch config back to default (software) and restart.
	auth1.process.Close()
	require.NoError(t, auth1.waitForShutdown(ctx))
	auth1Config.Auth.KeyStore = servicecfg.KeystoreConfig{}
	auth1, err = newTeleportService(ctx, auth1Config, "auth1")
	require.NoError(t, err)

	// Make sure a cluster alert is created.
	alerts, err := auth1.process.GetAuthServer().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{})
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	alert := alerts[0]
	assert.Equal(t, types.AlertSeverity_HIGH, alert.Spec.Severity)
	assert.Contains(t, alert.Spec.Message, "configured to use raw software keys")
	assert.Contains(t, alert.Spec.Message, "the following CAs do not contain any keys of that type:")
	assert.Contains(t, alert.Spec.Message, "The Auth Service is currently unable to sign certificates")

	rotate := func(caType types.CertAuthType, targetPhase string) error {
		return auth1.process.GetAuthServer().RotateCertAuthority(ctx, types.RotateRequest{
			Type:        caType,
			TargetPhase: targetPhase,
			Mode:        types.RotationModeManual,
		})
	}
	for _, caType := range types.CertAuthTypes {
		for _, targetPhase := range []string{
			types.RotationPhaseInit,
			types.RotationPhaseUpdateClients,
			types.RotationPhaseUpdateServers,
			types.RotationPhaseStandby,
		} {
			log.DebugContext(ctx, "sending rotation request", "phase", targetPhase, "ca", caType)
			if caType == types.HostCA {
				expectedEvent := service.TeleportCredentialsUpdatedEvent
				if targetPhase == types.RotationPhaseInit {
					expectedEvent = service.TeleportPhaseChangeEvent
				}
				require.NoError(t, auth1.waitingForNewEvent(ctx, expectedEvent, func() error {
					return rotate(caType, targetPhase)
				}))
			} else {
				require.NoError(t, rotate(caType, targetPhase))
			}
		}
	}

	// Make sure the cluster alert gets cleared.
	// Advance far enough for auth.runPeriodicOperations to call
	// auth.AutoRotateCertAuthorities which reconciles the alert state.
	clock.Advance(2 * defaults.HighResPollingPeriod)
	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		alerts, err = auth1.process.GetAuthServer().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{})
		assert.NoError(t, err)
		assert.Empty(t, alerts)

		// Keep advancing the clock to make sure the rotation ticker gets fired
		clock.Advance(2 * defaults.HighResPollingPeriod)
	}, 5*time.Second, 100*time.Millisecond)
}
