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
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// teleportService wraps a *service.TeleportProcess and sets up a goroutine to
// handle process reloads. You must always call waitForNewProcess or
// waitForRestart in for the new process after an expected reload to be picked
// up. Methods are not meant to be called concurrently on the same receiver and
// are not generally thread safe.
type teleportService struct {
	name              string
	log               utils.Logger
	config            *servicecfg.Config
	process           *service.TeleportProcess
	processGeneration int
	serviceChannel    chan *service.TeleportProcess
	errorChannel      chan error
}

func newTeleportService(t *testing.T, config *servicecfg.Config, name string) *teleportService {
	s := &teleportService{
		config:         config,
		name:           name,
		log:            config.Log,
		serviceChannel: make(chan *service.TeleportProcess, 1),
		errorChannel:   make(chan error, 1),
	}
	return s
}

func (t *teleportService) close() error {
	if t.process == nil {
		return nil
	}
	if err := t.process.Close(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(t.process.Wait())
}

func (t *teleportService) start(ctx context.Context) error {
	// Run the service in a background goroutine and hook into service.Run to
	// receive all new processes after restarts and write them to a goroutine.
	go func() {
		t.errorChannel <- service.Run(ctx, *t.config, func(cfg *servicecfg.Config) (service.Process, error) {
			t.log.Debugf("%s gen %d: starting next process generation (gen %d)", t.name, t.processGeneration, t.processGeneration+1)
			svc, err := service.NewTeleport(cfg)
			if err == nil {
				t.log.Debugf("%s gen %d: started, writing to serviceChannel", t.name, t.processGeneration+1)
				t.serviceChannel <- svc
			}
			return svc, trace.Wrap(err)
		})
	}()
	t.log.Debugf("%s gen 1: waiting for first start", t.name)
	if err := t.waitForNewProcess(ctx); err != nil {
		return trace.Wrap(err)
	}
	t.log.Debugf("%s gen 1: started, waiting for it to be ready", t.name)
	return t.waitForReady(ctx)
}

func (t *teleportService) waitForNewProcess(ctx context.Context) error {
	select {
	case t.process = <-t.serviceChannel:
		t.processGeneration += 1
		t.log.Debugf("%s gen %d: received new process from serviceChannel", t.name, t.processGeneration)
	case err := <-t.errorChannel:
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "%s gen %d: timed out waiting for restart", t.name, t.processGeneration)
	}
	return nil
}

func (t *teleportService) waitForEvent(ctx context.Context, event string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	waitForEventErr := make(chan error)
	go func() {
		_, err := t.process.WaitForEvent(ctx, event)
		select {
		case waitForEventErr <- err:
		case <-ctx.Done():
		}
	}()
	select {
	case err := <-waitForEventErr:
		return trace.Wrap(err)
	case err := <-t.errorChannel:
		if err != nil {
			return trace.Wrap(err, "process unexpectedly exited while waiting for event %s", event)
		}
		return trace.Errorf("process unexpectedly exited while waiting for event %s", event)
	case <-t.serviceChannel:
		return trace.Errorf("process unexpectedly reloaded while waiting for event %s", event)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

func (t *teleportService) waitForReady(ctx context.Context) error {
	t.log.Debugf("%s gen %d: waiting for TeleportReadyEvent", t.name, t.processGeneration)
	if err := t.waitForEvent(ctx, service.TeleportReadyEvent); err != nil {
		return trace.Wrap(err, "waiting for %s gen %d to be ready", t.name, t.processGeneration)
	}
	t.log.Debugf("%s gen %d: got TeleportReadyEvent", t.name, t.processGeneration)
	// If this is an Auth server, also wait for AuthIdentityEvent so that we
	// can safely read the admin credentials and create a test client.
	if t.process.GetAuthServer() != nil {
		t.log.Debugf("%s gen %d: waiting for AuthIdentityEvent", t.name, t.processGeneration)
		if err := t.waitForEvent(ctx, service.AuthIdentityEvent); err != nil {
			return trace.Wrap(err, "%s gen %d: timed out waiting AuthIdentityEvent", t.name, t.processGeneration)
		}
		t.log.Debugf("%s gen %d: got AuthIdentityEvent", t.name, t.processGeneration)
	}
	return nil
}

func (t *teleportService) waitForRestart(ctx context.Context) error {
	t.log.Debugf("%s gen %d: waiting for restart", t.name, t.processGeneration)
	if err := t.waitForNewProcess(ctx); err != nil {
		return trace.Wrap(err)
	}
	t.log.Debugf("%s gen %d: restarted, waiting for new process (gen %d) to be ready", t.name, t.processGeneration-1, t.processGeneration)
	return trace.Wrap(t.waitForReady(ctx))
}

func (t *teleportService) waitForShutdown(ctx context.Context) error {
	t.log.Debugf("%s gen %d: waiting for shutdown", t.name, t.processGeneration)
	select {
	case err := <-t.errorChannel:
		t.process = nil
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "%s gen %d: timed out waiting for shutdown", t.name, t.processGeneration)
	}
}

func (t *teleportService) waitForLocalAdditionalKeys(ctx context.Context) error {
	t.log.Debugf("%s gen %d: waiting for local additional keys", t.name, t.processGeneration)
	clusterName, err := t.process.GetAuthServer().GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	hostCAID := types.CertAuthID{DomainName: clusterName.GetClusterName(), Type: types.HostCA}
	for {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err(), "%s gen %d: timed out waiting for local additional keys", t.name, t.processGeneration)
		case <-time.After(250 * time.Millisecond):
		}
		ca, err := t.process.GetAuthServer().GetCertAuthority(ctx, hostCAID, true)
		if err != nil {
			return trace.Wrap(err)
		}
		usableKeysResult, err := t.process.GetAuthServer().GetKeyStore().HasUsableAdditionalKeys(ctx, ca)
		if err != nil {
			return trace.Wrap(err)
		}
		if usableKeysResult.CAHasPreferredKeyType {
			break
		}
	}
	t.log.Debugf("%s gen %d has local additional keys", t.name, t.processGeneration)
	return nil
}

func (t *teleportService) waitForPhaseChange(ctx context.Context) error {
	t.log.Debugf("%s gen %d: waiting for phase change", t.name, t.processGeneration)
	if err := t.waitForEvent(ctx, service.TeleportPhaseChangeEvent); err != nil {
		return trace.Wrap(err, "%s gen %d: timed out waiting for phase change", t.name, t.processGeneration)
	}
	t.log.Debugf("%s gen %d: changed phase", t.name, t.processGeneration)
	return nil
}

func (t *teleportService) authAddr(testingT *testing.T) utils.NetAddr {
	addr, err := t.process.AuthAddr()
	require.NoError(testingT, err)

	return *addr
}

func (t *teleportService) authAddrString(testingT *testing.T) string {
	addr, err := t.process.AuthAddr()
	require.NoError(testingT, err)
	return addr.String()
}

type teleportServices []*teleportService

func (s teleportServices) forEach(f func(t *teleportService) error) error {
	for i := range s {
		if err := f(s[i]); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (s teleportServices) start(ctx context.Context) error {
	return s.forEach(func(t *teleportService) error { return t.start(ctx) })
}

func (s teleportServices) waitForRestart(ctx context.Context) error {
	return s.forEach(func(t *teleportService) error { return t.waitForRestart(ctx) })
}

func (s teleportServices) waitForLocalAdditionalKeys(ctx context.Context) error {
	return s.forEach(func(t *teleportService) error { return t.waitForLocalAdditionalKeys(ctx) })
}

func (s teleportServices) waitForPhaseChange(ctx context.Context) error {
	return s.forEach(func(t *teleportService) error { return t.waitForPhaseChange(ctx) })
}

func newAuthConfig(t *testing.T, log utils.Logger) *servicecfg.Config {
	config := servicecfg.MakeDefaultConfig()
	config.DataDir = t.TempDir()
	config.Auth.StorageConfig.Params["path"] = filepath.Join(config.DataDir, defaults.BackendDir)
	config.SSH.Enabled = false
	config.Proxy.Enabled = false
	config.Log = log
	config.InstanceMetadataClient = imds.NewDisabledIMDSClient()
	config.MaxRetryPeriod = 25 * time.Millisecond
	config.PollingPeriod = 2 * time.Second
	config.Clock = fastClock(t)

	config.Auth.Enabled = true
	config.Auth.NoAudit = true
	config.Auth.ListenAddr.Addr = net.JoinHostPort("localhost", "0")
	config.Auth.PublicAddrs = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        "localhost",
		},
	}
	var err error
	config.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "test-cluster",
	})
	require.NoError(t, err)
	config.SetAuthServerAddress(config.Auth.ListenAddr)
	config.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Roles: []types.SystemRole{"Proxy", "Node"},
				Token: "foo",
			},
		},
	})
	require.NoError(t, err)

	return config
}

func newProxyConfig(t *testing.T, authAddr utils.NetAddr, log utils.Logger) *servicecfg.Config {
	config := servicecfg.MakeDefaultConfig()
	config.Version = defaults.TeleportConfigVersionV3
	config.DataDir = t.TempDir()
	config.CachePolicy.Enabled = true
	config.Auth.Enabled = false
	config.SSH.Enabled = false
	config.SetToken("foo")
	config.SetAuthServerAddress(authAddr)
	config.Log = log
	config.InstanceMetadataClient = imds.NewDisabledIMDSClient()
	config.MaxRetryPeriod = 25 * time.Millisecond
	config.PollingPeriod = 2 * time.Second
	config.Clock = fastClock(t)

	config.Proxy.Enabled = true
	config.Proxy.DisableWebInterface = true
	config.Proxy.DisableWebService = true
	config.Proxy.DisableReverseTunnel = true
	config.Proxy.SSHAddr.Addr = net.JoinHostPort("localhost", "0")
	config.Proxy.WebAddr.Addr = net.JoinHostPort("localhost", "0")
	config.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	return config
}

// fastClock returns a clock that runs at ~20x realtime.
func fastClock(t *testing.T) clockwork.FakeClock {
	// Start in the past to avoid cert not yet valid errors
	clock := clockwork.NewFakeClockAt(time.Now().Add(-12 * time.Hour))
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			clock.BlockUntil(1)
			clock.Advance(time.Second)
			time.Sleep(50 * time.Millisecond)
		}
	}()
	return clock
}
