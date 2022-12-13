/*
Copyright 2022 Gravitational, Inc.

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

package tbot

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/tbot/testhelpers"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	native.PrecomputeTestKeys(m)
	os.Exit(m.Run())
}

func rotate(
	ctx context.Context, t *testing.T, log logrus.FieldLogger, svc func() *service.TeleportProcess, phase string, reload chan struct{},
) {
	t.Helper()
	log.Infof("Triggering rotation: %s", phase)
	err := svc().GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
		// only rotate Host CA as to avoid race condition serverside when
		// multiple CAs are rotated at once and the database closes off.
		Type:        types.HostCA,
		Mode:        "manual",
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
			t.Logf("service reloaded")
		case <-time.After(10 * time.Second):
			require.FailNow(t, "failed waiting of the service restart")
		}

		err = waitForProcessEvent(svc(), service.TeleportReadyEvent, 30*time.Second)
		require.NoError(t, err)
	}

	cid := types.CertAuthID{Type: types.HostCA, DomainName: "ubuntu-22-04"}

	require.Eventually(t, func() bool {
		cert, err := svc().GetAuthServer().GetCertAuthority(ctx, cid, false)
		require.NoError(t, err)
		if err != nil {
			return false
		}
		return cert.GetRotation().Phase == phase
	}, 30*time.Second, 200*time.Millisecond)
}

// waitForProcessEvent waits for process event to occur or timeout
func waitForProcessEvent(svc *service.TeleportProcess, event string, timeout time.Duration) error {
	if _, err := svc.WaitForEventTimeout(timeout, event); err != nil {
		return trace.BadParameter("timeout waiting for service to broadcast event %v", event)
	}
	return nil
}

//// waitForReload waits for multiple events to happen:
////
//// 1. new service to be created and started
//// 2. old service, if present to shut down
////
//// this helper function allows to serialize tests for reloads.
//func waitForReload(serviceC chan *service.TeleportProcess, old *service.TeleportProcess) (*service.TeleportProcess, error) {
//	var svc *service.TeleportProcess
//	select {
//	case svc = <-serviceC:
//	case <-time.After(1 * time.Minute):
//		//dumpGoroutineProfile()
//		return nil, trace.BadParameter("timeout waiting for service to start")
//	}
//
//	if _, err := svc.WaitForEventTimeout(20*time.Second, service.TeleportReadyEvent); err != nil {
//		//dumpGoroutineProfile()
//		return nil, trace.BadParameter("timeout waiting for service to broadcast ready status")
//	}
//
//	// if old service is present, wait for it to complete shut down procedure
//	if old != nil {
//		ctx, cancel := context.WithCancel(context.TODO())
//		go func() {
//			defer cancel()
//			old.Supervisor.Wait()
//		}()
//		select {
//		case <-ctx.Done():
//		case <-time.After(1 * time.Minute):
//			//dumpGoroutineProfile()
//			return nil, trace.BadParameter("timeout waiting for old service to stop")
//		}
//	}
//	return svc, nil
//}

func setupServerForCARotationTest(ctx context.Context, log utils.Logger, t *testing.T,
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
		//require.NoError(t, err)
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

	reloadChan := make(chan struct{})
	// Tracks the latest instance of the Teleport service through reloads
	activeSvc := svc
	activeSvcMu := sync.Mutex{}
	go func() {
		defer func() {
			errC <- nil
		}()

		for {
			select {
			case svc := <-svcC:
				activeSvcMu.Lock()
				activeSvc = svc
				activeSvcMu.Unlock()
				reloadChan <- struct{}{}
			case <-ctx.Done():
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

	errC := make(chan error)
	t.Cleanup(func() {
		log.Infof("Shutting down long running test processes..")
		cancel()

		for i := 0; i < 3; i++ {
			require.NoError(t, <-errC)
		}
	})

	client, teleportProcess, fc, reloadChan := setupServerForCARotationTest(ctx, log, t, errC)

	// Make and join a new bot instance.
	botParams := testhelpers.MakeBot(t, client, "test", "access")
	botConfig := testhelpers.MakeMemoryBotConfig(t, fc, botParams)
	botConfig.RenewalInterval = 5 * time.Second
	b := New(botConfig, log, make(chan struct{}))

	go func() {
		err := b.Run(ctx)
		errC <- err
	}()

	// Allow time for bot to start running and watching for CA rotations
	// TODO: We should modify the bot to emit events that may be useful...
	//time.Sleep(10 * time.Second)

	require.Eventually(t, func() bool {
		return b.ident() != nil
	}, 10*time.Second, 200*time.Millisecond)

	// fetch initial host cert
	require.Len(t, b.ident().TLSCACertsBytes, 2)
	initialCAs := [][]byte{}
	copy(initialCAs, b.ident().TLSCACertsBytes)

	// Begin rotating through all the phases, testing the client after
	// each rotation phase has completed.
	rotate(ctx, t, log, teleportProcess, types.RotationPhaseInit, reloadChan)
	// TODO: These sleeps allow the client time to rotate. They could be
	// replaced if tbot emitted a CA rotation/renewal event.
	//time.Sleep(time.Second * 30)
	_, err := b.Client().Ping(ctx)
	require.NoError(t, err)

	rotate(ctx, t, log, teleportProcess, types.RotationPhaseUpdateClients, reloadChan)
	//time.Sleep(time.Second * 30)
	// Ensure both sets of CA certificates are now available locally
	require.Eventually(t, func() bool {
		return len(b.ident().TLSCACertsBytes) == 3
	}, 30*time.Second, 200*time.Millisecond)

	//require.Len(t, b.ident().TLSCACertsBytes, 3)
	resp, err := b.Client().Ping(ctx)
	require.NoError(t, err)
	require.NotEqual(t, "", resp.ServerVersion)

	rotate(ctx, t, log, teleportProcess, types.RotationPhaseUpdateServers, reloadChan)
	//time.Sleep(time.Second * 30)
	//_, err = b.Client().GetCertAuthority(ctx, types.CertAuthID{
	//	Type:       types.HostCA,
	//	DomainName: "ubuntu-22-04",
	//}, false)
	//require.NoError(t, err)
	//require.NotEqual(t, "", resp.ServerVersion)
	//b.Client().

	rotate(ctx, t, log, teleportProcess, types.RotationStateStandby, reloadChan)
	//b.Client().DeleteAllLocks()
	//time.Sleep(time.Second * 30)
	_, err = b.Client().Ping(ctx)
	require.NoError(t, err)
	require.NotEqual(t, "", resp.ServerVersion)

	require.Eventually(t, func() bool {
		return len(b.ident().TLSCACertsBytes) == 2
	}, 30*time.Second, 200*time.Millisecond)

	require.Len(t, b.ident().TLSCACertsBytes, 2)
	finalCAs := b.ident().TLSCACertsBytes
	require.NotEqual(t, initialCAs, finalCAs)
}
