/*
 *
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/tbot"
	"github.com/gravitational/teleport/lib/tbot/testhelpers"
	"github.com/gravitational/teleport/lib/utils"
)

func rotate(
	ctx context.Context, t *testing.T, log logrus.FieldLogger, svc func() *service.TeleportProcess, phase string, reload chan struct{},
) {
	t.Helper()

	log.Infof("Triggering rotation: %s", phase)
	err := svc().GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
		// only rotate Host CA as to avoid race condition serverside when
		// multiple CAs are rotated at once and the database closes off.
		Type:        types.HostCA,
		Mode:        types.RotationModeManual,
		TargetPhase: phase,
	})
	if err != nil {
		log.WithError(err).Infof("Error occurred during triggering rotation: %s", phase)
	}
	require.NoError(t, err)
	log.Infof("Triggered rotation: %s", phase)

	if phase == types.RotationPhaseInit {
		err = waitForProcessEvent(svc(), service.TeleportPhaseChangeEvent, 30*time.Second)
		require.NoError(t, err)
	} else {
		err = waitForProcessEvent(svc(), service.TeleportReloadEvent, 30*time.Second)
		require.NoError(t, err)

		select {
		case <-reload:
		case <-time.After(20 * time.Second):
			require.FailNow(t, "failed waiting for the service restart")
		}
	}

	err = waitForProcessEvent(svc(), service.TeleportReadyEvent, 30*time.Second)
	require.NoError(t, err)

	err = waitForProcessEvent(svc(), service.ProxyReverseTunnelReady, 30*time.Second)
	require.NoError(t, err)
}

func setupServerForCARotationTest(ctx context.Context, t *testing.T, log utils.Logger,
	errC chan error) (auth.ClientI, func() *service.TeleportProcess, *config.FileConfig, chan struct{},
) {
	fc, fds := testhelpers.DefaultConfig(t)

	cfg := service.MakeDefaultConfig()
	require.NoError(t, config.ApplyFileConfig(fc, cfg))
	cfg.FileDescriptors = fds
	cfg.Log = log
	cfg.CachePolicy.Enabled = false
	cfg.Proxy.DisableWebInterface = true
	cfg.RotationConnectionInterval = 500 * time.Millisecond
	cfg.PollingPeriod = time.Second

	svcC := make(chan *service.TeleportProcess)
	go func() {
		err := service.Run(ctx, *cfg, func(cfg *service.Config) (service.Process, error) {
			svc, err := service.NewTeleport(cfg)
			if err == nil {
				svcC <- svc
			}
			return svc, err
		})
		errC <- err
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

	// reloadChan is a blocking channel that releases after each service reload.
	reloadChan := make(chan struct{})
	// Tracks the latest instance of the Teleport service through reloads
	activeSvc := svc
	activeSvcMu := sync.Mutex{}
	go func() {
		for {
			select {
			case svc := <-svcC:
				activeSvcMu.Lock()
				activeSvc = svc
				activeSvcMu.Unlock()
				reloadChan <- struct{}{}
			case <-ctx.Done():
				if ctx.Err() != context.Canceled {
					errC <- ctx.Err()
				} else {
					// Push nil to unlock the channel.
					errC <- nil
				}
				return
			}
		}
	}()

	return testhelpers.MakeDefaultAuthClient(t, log, fc), func() *service.TeleportProcess {
		activeSvcMu.Lock()
		defer activeSvcMu.Unlock()
		return activeSvc
	}, fc, reloadChan
}

// TestCARotation is a heavy integration test that through a rotation, the bot
// receives credentials for a new CA.
func TestBot_Run_CARotation(t *testing.T) {
	t.Parallel()

	// wg and context manage the cancellation of long-running processes e.g.
	// teleport and tbot in the test.
	log := utils.NewLoggerForTests()
	ctx, cancel := context.WithCancel(context.Background())

	// errC accumulates errors from all created goroutines in the test.
	errC := make(chan error)
	t.Cleanup(func() {
		log.Infof("Shutting down long running test processes..")
		cancel()

		for i := 0; i < 3; i++ {
			require.NoError(t, <-errC)
		}
	})

	client, teleportProcess, fc, reloadChan := setupServerForCARotationTest(ctx, t, log, errC)

	// Make and join a new bot instance.
	botParams := testhelpers.MakeBot(t, client, "test", "access")
	botConfig := testhelpers.MakeMemoryBotConfig(t, fc, botParams)
	// Set RenewalInterval to poll for status changes more often.
	// Setting it below 2s doesn't seem to improve the runtime. Values lower
	// than 4s makes this test flaky.
	// TODO(jakule): Changing this value should not make this test flaky.
	botConfig.RenewalInterval = 4 * time.Second

	testBot := tbot.TestBot{
		Bot: tbot.New(botConfig, log, make(chan struct{})),
	}

	go func() {
		err := testBot.Run(ctx)
		errC <- err
	}()

	// Wait for the bot to start running and watching for CA rotations.
	require.Eventually(t, func() bool {
		return testBot.Ident(t) != nil
	}, 10*time.Second, 200*time.Millisecond)

	// Fetch initial host cert
	require.Len(t, testBot.Ident(t).TLSCACertsBytes, 2)
	initialCAs := [][]byte{}
	copy(initialCAs, testBot.Ident(t).TLSCACertsBytes)

	// Begin rotating through all the phases, testing the client after
	// each rotation phase has completed.
	rotate(ctx, t, log, teleportProcess, types.RotationPhaseInit, reloadChan)

	rotate(ctx, t, log, teleportProcess, types.RotationPhaseUpdateClients, reloadChan)

	// Ensure both sets of CA certificates are now available locally
	require.Eventually(t, func() bool {
		return len(testBot.Ident(t).TLSCACertsBytes) == 3
	}, 30*time.Second, 200*time.Millisecond)

	rotate(ctx, t, log, teleportProcess, types.RotationPhaseUpdateServers, reloadChan)

	rotate(ctx, t, log, teleportProcess, types.RotationStateStandby, reloadChan)

	require.Eventually(t, func() bool {
		return len(testBot.Ident(t).TLSCACertsBytes) == 2
	}, 30*time.Second, 200*time.Millisecond)

	finalCAs := testBot.Ident(t).TLSCACertsBytes
	require.NotEqual(t, initialCAs, finalCAs)
}
