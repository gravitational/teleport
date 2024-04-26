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

package tbot

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tbot/testhelpers"
	"github.com/gravitational/teleport/lib/utils"
)

func Test_filterCAEvent(t *testing.T) {
	clusterName := "example.com"
	createCertAuthority := func(t *testing.T, modifier func(spec *types.CertAuthoritySpecV2)) types.CertAuthority {
		t.Helper()
		validSpec := types.CertAuthoritySpecV2{
			ClusterName: clusterName,
			Type:        "host",
			Rotation: &types.Rotation{
				Phase: "update_clients",
			},
		}

		if modifier != nil {
			modifier(&validSpec)
		}

		ca, err := types.NewCertAuthority(validSpec)
		require.NoError(t, err)
		return ca
	}

	tests := []struct {
		name                 string
		event                types.Event
		expectedIgnoreReason string
	}{
		{
			name: "valid host CA rotation",
			event: types.Event{
				Type:     types.OpPut,
				Resource: createCertAuthority(t, nil),
			},
		},
		{
			name: "valid user CA rotation",
			event: types.Event{
				Type: types.OpPut,
				Resource: createCertAuthority(t, func(spec *types.CertAuthoritySpecV2) {
					spec.Type = "user"
				}),
			},
		},
		{
			name: "valid DB CA rotation",
			event: types.Event{
				Type: types.OpPut,
				Resource: createCertAuthority(t, func(spec *types.CertAuthoritySpecV2) {
					spec.Type = "db"
				}),
			},
		},
		{
			name: "wrong type",
			event: types.Event{
				Type:     types.OpDelete,
				Resource: createCertAuthority(t, nil),
			},
			expectedIgnoreReason: "type not PUT",
		},
		{
			name: "wrong underlying resource",
			event: types.Event{
				Type:     types.OpPut,
				Resource: &types.Namespace{},
			},
			expectedIgnoreReason: "event resource was not CertAuthority (*types.Namespace)",
		},
		{
			name: "wrong phase",
			event: types.Event{
				Type: types.OpPut,
				Resource: createCertAuthority(t, func(spec *types.CertAuthoritySpecV2) {
					spec.Rotation.Phase = "init"
				}),
			},
			expectedIgnoreReason: "skipping due to phase 'init'",
		},
		{
			name: "wrong cluster name",
			event: types.Event{
				Type: types.OpPut,
				Resource: createCertAuthority(t, func(spec *types.CertAuthoritySpecV2) {
					spec.ClusterName = "wrong"
				}),
			},
			expectedIgnoreReason: "skipping due to cluster name of CA: was 'wrong', wanted 'example.com'",
		},
		{
			name: "wrong CA type",
			event: types.Event{
				Type: types.OpPut,
				Resource: createCertAuthority(t, func(spec *types.CertAuthoritySpecV2) {
					spec.Type = "jwt"
				}),
			},
			expectedIgnoreReason: "skipping due to CA kind 'jwt'",
		},
	}

	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ignoreReason := filterCAEvent(ctx, log, tt.event, clusterName)
			require.Equal(t, tt.expectedIgnoreReason, ignoreReason)
		})
	}
}

func TestChannelBroadcaster(t *testing.T) {
	cb := channelBroadcaster{chanSet: map[chan struct{}]struct{}{}}
	sub1, unsubscribe1 := cb.subscribe()
	t.Cleanup(unsubscribe1)
	sub2, unsubscribe2 := cb.subscribe()
	t.Cleanup(unsubscribe2)

	cb.broadcast()
	require.NotEmpty(t, sub1)
	require.NotEmpty(t, sub2)

	// remove value from sub1 to check that if sub2 is full broadcasting still
	// works
	<-sub1
	cb.broadcast()
	require.NotEmpty(t, sub1)

	// empty out both channels and ensure unsubscribing means they no longer
	// receive values
	<-sub1
	<-sub2
	unsubscribe1()
	unsubscribe2()
	cb.broadcast()
	require.Empty(t, sub1)
	require.Empty(t, sub2)

	// ensure unsubscribing twice doesn't cause panic
	unsubscribe1()
}

func rotate( //nolint:unused // used in skipped test
	ctx context.Context, t *testing.T, log *slog.Logger, svc *service.TeleportProcess, phase string,
) {
	t.Helper()
	log.InfoContext(ctx, "Triggering rotation", "phase", phase)
	err := svc.GetAuthServer().RotateCertAuthority(ctx, types.RotateRequest{
		// only rotate Host CA as to avoid race condition serverside when
		// multiple CAs are rotated at once and the database closes off.
		Type:        types.HostCA,
		Mode:        "manual",
		TargetPhase: phase,
	})
	if err != nil {
		log.InfoContext(
			ctx,
			"Error occurred during triggering rotation",
			"phase", phase,
			"error", err,
		)
	}
	require.NoError(t, err)
	log.InfoContext(ctx, "Triggered rotation: %s", "phase", phase)
}

func setupServerForCARotationTest(
	ctx context.Context,
	log *slog.Logger,
	t *testing.T,
	wg *sync.WaitGroup,
	//nolint:unused // used in skipped test
) (*auth.Client, func() *service.TeleportProcess, *config.FileConfig) {
	fc, fds := testhelpers.DefaultConfig(t)

	cfg := servicecfg.MakeDefaultConfig()
	require.NoError(t, config.ApplyFileConfig(fc, cfg))
	cfg.FileDescriptors = fds
	cfg.Logger = log
	cfg.CachePolicy.Enabled = false
	cfg.Proxy.DisableWebInterface = true
	cfg.InstanceMetadataClient = cloud.NewDisabledIMDSClient()

	svcC := make(chan *service.TeleportProcess)
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := service.Run(ctx, *cfg, func(cfg *servicecfg.Config) (service.Process, error) {
			svc, err := service.NewTeleport(cfg)
			if err == nil {
				svcC <- svc
			}
			return svc, err
		})
		require.NoError(t, err)
	}()

	var svc *service.TeleportProcess
	select {
	case <-time.After(30 * time.Second):
		// this should really happen quite quickly, but under the load during
		// parallel test run, it can take a while.
		t.Fatal("teleport process did not instantiate in 30 seconds")
	case svc = <-svcC:
	}

	// Ensure the service starts correctly the first time before proceeding
	_, err := svc.WaitForEventTimeout(30*time.Second, service.TeleportReadyEvent)
	// in reality, the auth server should start *much* sooner than this.  we use a very large
	// timeout here because this isn't the kind of problem that this test is meant to catch.
	require.NoError(t, err, "auth server didn't start after 30s")

	// Tracks the latest instance of the Teleport service through reloads
	activeSvc := svc
	activeSvcMu := sync.Mutex{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case svc := <-svcC:
				activeSvcMu.Lock()
				activeSvc = svc
				activeSvcMu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	return testhelpers.MakeDefaultAuthClient(t, fc), func() *service.TeleportProcess {
		activeSvcMu.Lock()
		defer activeSvcMu.Unlock()
		return activeSvc
	}, fc
}

// TestCARotation is a heavy integration test that through a rotation, the bot
// receives credentials for a new CA.
func TestBot_Run_CARotation(t *testing.T) {
	// TODO(jakule): Re-enable this test https://github.com/gravitational/teleport/issues/19403
	t.Skip("Temporary disable until it's fixed - flaky")

	t.Parallel()
	if testing.Short() {
		t.Skip("test skipped when -short provided")
	}

	// wg and context manage the cancellation of long running processes e.g
	// teleport and tbot in the test.
	log := utils.NewSlogLoggerForTests()
	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		log.InfoContext(ctx, "Shutting down long running test processes..")
		cancel()
		wg.Wait()
	})

	client, teleportProcess, fc := setupServerForCARotationTest(ctx, log, t, wg)

	// Make and join a new bot instance.
	botParams, _ := testhelpers.MakeBot(
		t, client, "test", "access",
	)
	botConfig := testhelpers.DefaultBotConfig(t, fc, botParams, nil,
		testhelpers.DefaultBotConfigOpts{
			UseAuthServer: true,
			Insecure:      true,
		},
	)
	b := New(botConfig, log)

	resolver, err := reversetunnelclient.CachingResolver(
		ctx,
		reversetunnelclient.WebClientResolver(&webclient.Config{
			Context:   ctx,
			ProxyAddr: b.cfg.AuthServer,
			Insecure:  b.cfg.Insecure,
		}),
		nil /* clock */)
	require.NoError(t, err, "creating tunnel resolver")

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := b.Run(ctx)
		assert.NoError(t, err)
	}()
	// Allow time for bot to start running and watching for CA rotations
	// TODO: We should modify the bot to emit events that may be useful...
	time.Sleep(10 * time.Second)
	facade := b.botIdentitySvc.facade

	// fetch initial host cert
	require.Len(t, b.BotIdentity().TLSCACertsBytes, 2)
	var initialCAs [][]byte
	copy(initialCAs, b.BotIdentity().TLSCACertsBytes)

	// Begin rotating through all of the phases, testing the client after
	// each rotation phase has completed.
	rotate(ctx, t, log, teleportProcess(), types.RotationPhaseInit)
	// TODO: These sleeps allow the client time to rotate. They could be
	// replaced if tbot emitted a CA rotation/renewal event.
	time.Sleep(time.Second * 30)

	_, err = clientForFacade(ctx, log, botConfig, facade, resolver)
	require.NoError(t, err)

	rotate(ctx, t, log, teleportProcess(), types.RotationPhaseUpdateClients)
	time.Sleep(time.Second * 30)
	// Ensure both sets of CA certificates are now available locally
	require.Len(t, b.BotIdentity().TLSCACertsBytes, 3)
	_, err = clientForFacade(ctx, log, botConfig, facade, resolver)
	require.NoError(t, err)

	rotate(ctx, t, log, teleportProcess(), types.RotationPhaseUpdateServers)
	time.Sleep(time.Second * 30)
	_, err = clientForFacade(ctx, log, botConfig, facade, resolver)
	require.NoError(t, err)

	rotate(ctx, t, log, teleportProcess(), types.RotationStateStandby)
	time.Sleep(time.Second * 30)
	_, err = clientForFacade(ctx, log, botConfig, facade, resolver)
	require.NoError(t, err)

	require.Len(t, b.BotIdentity().TLSCACertsBytes, 2)
	finalCAs := b.BotIdentity().TLSCACertsBytes
	require.NotEqual(t, initialCAs, finalCAs)
}
