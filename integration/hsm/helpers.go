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
	name    string
	log     *slog.Logger
	config  *servicecfg.Config
	process *service.TeleportProcess

	errC chan struct{}
	err  error
}

func newTeleportService(ctx context.Context, config *servicecfg.Config, name string) (*teleportService, error) {
	t := &teleportService{
		name:   name,
		log:    config.Logger.With("helper_service", name),
		config: config,
		errC:   make(chan struct{}),
	}
	svc, err := service.NewTeleport(t.config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := svc.Start(); err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer close(t.errC)
		t.err = svc.Wait()
	}()
	t.process = svc

	if _, err := t.process.WaitForEvent(ctx, service.TeleportReadyEvent); err != nil {
		return nil, trace.Wrap(err)
	}

	if t.process.GetAuthServer() != nil {
		if _, err := t.process.WaitForEvent(ctx, service.AuthIdentityEvent); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return t, nil
}

func (t *teleportService) waitForShutdown(ctx context.Context) error {
	select {
	case <-t.errC:
		return trace.Wrap(t.err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "%v: timed out waiting for shutdown", t.name)
	}
}

func (t *teleportService) waitForLocalAdditionalKeys(ctx context.Context) error {
	t.log.DebugContext(ctx, "waiting for local additional keys")
	authServer := t.process.GetAuthServer()
	if authServer == nil {
		return trace.NotFound("%v: attempted to wait for additional keys in a service with no auth", t.name)
	}

	clusterName, err := authServer.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	hostCAID := types.CertAuthID{
		DomainName: clusterName.GetClusterName(),
		Type:       types.HostCA,
	}

	for {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err(), "%s: timed out waiting for local additional keys", t.name)
		case <-time.After(250 * time.Millisecond):
		}

		ca, err := authServer.GetCertAuthority(ctx, hostCAID, true)
		if err != nil {
			return trace.Wrap(err)
		}
		usableKeysResult, err := authServer.GetKeyStore().HasUsableAdditionalKeys(ctx, ca)
		if err != nil {
			return trace.Wrap(err)
		}
		if usableKeysResult.CAHasPreferredKeyType {
			t.log.DebugContext(ctx, "got local additional keys")
			return nil
		}
	}
}

// waitingForNewEvent starts listening for the named event, runs the given
// closure, then waits for the event to be emitted.
func (t *teleportService) waitingForNewEvent(ctx context.Context, name string, fn func() error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	evC := make(chan service.Event, 1)
	t.process.ListenForNewEvents(ctx, name, evC)

	if err := fn(); err != nil {
		return trace.Wrap(err)
	}

	select {
	case <-evC:
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
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

func (s teleportServices) waitingForNewEvent(ctx context.Context, name string, fn func() error) error {
	// assuming two services (which is all we ever use this for anyway), the
	// following loop results in this, which is exactly what we'd write to wait
	// for the same event in two services ([teleportService.waitingForNewEvent]
	// wraps a closure and we want to call it once, so we can't just use
	// [teleportServices.forEach])
	//
	//	svc1.waitingForNewEvent(ctx, name, func() error {
	//		return svc2.waitingForNewEvent(ctx, name, fn)
	//	})

	for _, svc := range s {
		oldFn := fn
		fn = func() error {
			return svc.waitingForNewEvent(ctx, name, oldFn)
		}
	}
	return fn()
}

func (s teleportServices) waitForLocalAdditionalKeys(ctx context.Context) error {
	return s.forEach(func(t *teleportService) error { return t.waitForLocalAdditionalKeys(ctx) })
}

func newAuthConfig(t *testing.T, log *slog.Logger, clock clockwork.Clock) *servicecfg.Config {
	config := servicecfg.MakeDefaultConfig()
	config.DataDir = t.TempDir()
	config.Auth.StorageConfig.Params["path"] = filepath.Join(config.DataDir, defaults.BackendDir)
	config.SSH.Enabled = false
	config.Proxy.Enabled = false
	config.Logger = log
	config.InstanceMetadataClient = imds.NewDisabledIMDSClient()
	config.MaxRetryPeriod = 25 * time.Millisecond
	config.PollingPeriod = 2 * time.Second
	config.Clock = clock

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

func newProxyConfig(t *testing.T, authAddr utils.NetAddr, log *slog.Logger, clock clockwork.Clock) *servicecfg.Config {
	config := servicecfg.MakeDefaultConfig()
	config.Version = defaults.TeleportConfigVersionV3
	config.DataDir = t.TempDir()
	config.CachePolicy.Enabled = true
	config.Auth.Enabled = false
	config.SSH.Enabled = false
	config.SetToken("foo")
	config.SetAuthServerAddress(authAddr)
	config.Logger = log
	config.InstanceMetadataClient = imds.NewDisabledIMDSClient()
	config.MaxRetryPeriod = 25 * time.Millisecond
	config.PollingPeriod = 2 * time.Second
	config.Clock = clock

	config.Proxy.Enabled = true
	config.Proxy.DisableWebInterface = true
	config.Proxy.DisableWebService = true
	config.Proxy.DisableReverseTunnel = true
	config.Proxy.SSHAddr.Addr = net.JoinHostPort("localhost", "0")
	config.Proxy.WebAddr.Addr = net.JoinHostPort("localhost", "0")
	config.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	return config
}
