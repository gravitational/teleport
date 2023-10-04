// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hsm

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
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
	name           string
	log            utils.Logger
	config         *servicecfg.Config
	process        *service.TeleportProcess
	serviceChannel chan *service.TeleportProcess
	errorChannel   chan error
}

func newTeleportService(t *testing.T, config *servicecfg.Config, name string) *teleportService {
	s := &teleportService{
		config:         config,
		name:           name,
		log:            config.Log,
		serviceChannel: make(chan *service.TeleportProcess, 1),
		errorChannel:   make(chan error, 1),
	}
	t.Cleanup(func() {
		require.NoError(t, s.close(), "error while closing %s during test cleanup", name)
	})
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
			t.log.Debugf("(Re)starting %s", t.name)
			svc, err := service.NewTeleport(cfg)
			if err == nil {
				t.log.Debugf("Started %s, writing to serviceChannel", t.name)
				t.serviceChannel <- svc
			}
			return svc, trace.Wrap(err)
		})
	}()
	t.log.Debugf("Waiting for %s to start", t.name)
	if err := t.waitForNewProcess(ctx); err != nil {
		return trace.Wrap(err)
	}
	t.log.Debugf("%s started, waiting for it to be ready", t.name)
	return t.waitForReady(ctx)
}

func (t *teleportService) waitForNewProcess(ctx context.Context) error {
	select {
	case t.process = <-t.serviceChannel:
		t.log.Debugf("received new process for %s from serviceChannel", t.name)
	case err := <-t.errorChannel:
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "timed out waiting for %s to restart", t.name)
	}
	return nil
}

func (t *teleportService) waitForReady(ctx context.Context) error {
	t.log.Debugf("Waiting for %s to be ready", t.name)
	if _, err := t.process.WaitForEvent(ctx, service.TeleportReadyEvent); err != nil {
		return trace.Wrap(err, "timed out waiting for %s to be ready", t.name)
	}
	// If this is an Auth servier, also wait for AuthIdentityEvent so that we
	// can safely read the admin credentials and create a test client.
	if t.process.GetAuthServer() != nil {
		if _, err := t.process.WaitForEvent(ctx, service.AuthIdentityEvent); err != nil {
			return trace.Wrap(err, "timed out waiting for %s auth identity event", t.name)
		}
		t.log.Debugf("%s is ready", t.name)
	}
	return nil
}

func (t *teleportService) waitForRestart(ctx context.Context) error {
	t.log.Debugf("Waiting for %s to restart", t.name)
	if err := t.waitForNewProcess(ctx); err != nil {
		return trace.Wrap(err)
	}
	t.log.Debugf("%s restarted, waiting for new process to be ready", t.name)
	return trace.Wrap(t.waitForReady(ctx))
}

func (t *teleportService) waitForShutdown(ctx context.Context) error {
	t.log.Debugf("Waiting for %s to shut down", t.name)
	select {
	case err := <-t.errorChannel:
		t.process = nil
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "timed out waiting for %s to shut down", t.name)
	}
}

func (t *teleportService) waitForLocalAdditionalKeys(ctx context.Context) error {
	t.log.Debugf("Waiting for %s to have local additional keys", t.name)
	clusterName, err := t.process.GetAuthServer().GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	hostCAID := types.CertAuthID{DomainName: clusterName.GetClusterName(), Type: types.HostCA}
	for {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err(), "timed out waiting for %s to have local additional keys", t.name)
		case <-time.After(250 * time.Millisecond):
		}
		ca, err := t.process.GetAuthServer().GetCertAuthority(ctx, hostCAID, true)
		if err != nil {
			return trace.Wrap(err)
		}
		hasUsableKeys, err := t.process.GetAuthServer().GetKeyStore().HasUsableAdditionalKeys(ctx, ca)
		if err != nil {
			return trace.Wrap(err)
		}
		if hasUsableKeys {
			break
		}
	}
	t.log.Debugf("%s has local additional keys", t.name)
	return nil
}

func (t *teleportService) waitForPhaseChange(ctx context.Context) error {
	t.log.Debugf("Waiting for %s to change phase", t.name)
	if _, err := t.process.WaitForEvent(ctx, service.TeleportPhaseChangeEvent); err != nil {
		return trace.Wrap(err, "timed out waiting for %s to change phase", t.name)
	}
	t.log.Debugf("%s changed phase", t.name)
	return nil
}

func (t *teleportService) authAddr(testingT *testing.T) utils.NetAddr {
	addr, err := t.process.AuthAddr()
	require.NoError(testingT, err)

	return *addr
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
	hostName, err := os.Hostname()
	require.NoError(t, err)

	config := servicecfg.MakeDefaultConfig()
	config.DataDir = t.TempDir()
	config.Auth.StorageConfig.Params["path"] = filepath.Join(config.DataDir, defaults.BackendDir)
	config.SSH.Enabled = false
	config.Proxy.Enabled = false
	config.Log = log
	config.InstanceMetadataClient = cloud.NewDisabledIMDSClient()
	config.MaxRetryPeriod = 25 * time.Millisecond

	config.Auth.Enabled = true
	config.Auth.NoAudit = true
	config.Auth.ListenAddr.Addr = net.JoinHostPort(hostName, "0")
	config.Auth.PublicAddrs = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        hostName,
		},
	}
	config.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "testcluster",
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
	hostName, err := os.Hostname()
	require.NoError(t, err)

	config := servicecfg.MakeDefaultConfig()
	config.DataDir = t.TempDir()
	config.CachePolicy.Enabled = true
	config.Auth.Enabled = false
	config.SSH.Enabled = false
	config.SetToken("foo")
	config.SetAuthServerAddress(authAddr)
	config.Log = log
	config.InstanceMetadataClient = cloud.NewDisabledIMDSClient()
	config.MaxRetryPeriod = 25 * time.Millisecond

	config.Proxy.Enabled = true
	config.Proxy.DisableWebInterface = true
	config.Proxy.DisableWebService = true
	config.Proxy.DisableReverseTunnel = true
	config.Proxy.SSHAddr.Addr = net.JoinHostPort(hostName, "0")
	config.Proxy.WebAddr.Addr = net.JoinHostPort(hostName, "0")

	return config
}
