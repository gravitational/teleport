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

package reversetunnel

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/utils"
)

// RemoteClusterTunnelManager manages AgentPools for trusted (remote) clusters. It
// polls the auth server for ReverseTunnel resources and spins AgentPools for
// each one as needed.
//
// Note: ReverseTunnel resources on the auth server represent the desired
// tunnels, not actually active ones.
type RemoteClusterTunnelManager struct {
	cfg RemoteClusterTunnelManagerConfig

	mu      sync.Mutex
	pools   map[remoteClusterKey]*AgentPool
	stopRun func()
	retry   retryutils.Retry

	newAgentPool func(ctx context.Context, cfg RemoteClusterTunnelManagerConfig, cluster, addr string) (*AgentPool, error)
}

type remoteClusterKey struct {
	cluster string
	addr    string
}

// RemoteClusterTunnelManagerConfig is a bundle of parameters used by a
// RemoteClusterTunnelManager.
type RemoteClusterTunnelManagerConfig struct {
	// AuthClient is client to the auth server.
	AuthClient authclient.ClientI
	// AccessPoint is a lightweight access point that can optionally cache some
	// values.
	AccessPoint authclient.ProxyAccessPoint
	// AuthMethods contains SSH credentials that this pool connects as.
	AuthMethods []ssh.AuthMethod
	// HostUUID is a unique ID of this host
	HostUUID string
	// LocalCluster is a cluster name this client is a member of.
	LocalCluster string
	// Local ReverseTunnelServer to reach other cluster members connecting to
	// this proxy over a tunnel.
	ReverseTunnelServer reversetunnelclient.Server
	// KubeDialAddr is an optional address of a local kubernetes proxy.
	KubeDialAddr utils.NetAddr
	// FIPS indicates if Teleport was started in FIPS mode.
	FIPS bool
	// Logger is the logger
	Logger *slog.Logger
	// LocalAuthAddresses is a list of auth servers to use when dialing back to
	// the local cluster.
	LocalAuthAddresses []string
	// PROXYSigner is used to sign PROXY headers for securely propagating client IP address
	PROXYSigner multiplexer.PROXYHeaderSigner
}

func (c *RemoteClusterTunnelManagerConfig) CheckAndSetDefaults() error {
	if c.AuthClient == nil {
		return trace.BadParameter("missing AuthClient in RemoteClusterTunnelManagerConfig")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing AccessPoint in RemoteClusterTunnelManagerConfig")
	}
	if len(c.AuthMethods) == 0 {
		return trace.BadParameter("missing AuthMethods in RemoteClusterTunnelManagerConfig")
	}
	if c.HostUUID == "" {
		return trace.BadParameter("missing HostUUID in RemoteClusterTunnelManagerConfig")
	}
	if c.LocalCluster == "" {
		return trace.BadParameter("missing LocalCluster in RemoteClusterTunnelManagerConfig")
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}

	return nil
}

// NewRemoteClusterTunnelManager creates a new stopped tunnel manager with
// the provided config. Call Run() to start the manager.
func NewRemoteClusterTunnelManager(cfg RemoteClusterTunnelManagerConfig) (*RemoteClusterTunnelManager, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewLinearDriver(defaults.HighResPollingPeriod / 5),
		First:  retryutils.HalfJitter(defaults.HighResPollingPeriod),
		Max:    defaults.HighResPollingPeriod,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	w := &RemoteClusterTunnelManager{
		cfg:          cfg,
		pools:        make(map[remoteClusterKey]*AgentPool),
		newAgentPool: realNewAgentPool,
		retry:        retry,
	}
	return w, nil
}

// Close cleans up all outbound tunnels and stops the manager.
func (w *RemoteClusterTunnelManager) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for k, pool := range w.pools {
		pool.Stop()
		delete(w.pools, k)
	}
	if w.stopRun != nil {
		w.stopRun()
		w.stopRun = nil
	}

	return nil
}

// Run runs the manager polling loop. Run is blocking, start it in a goroutine.
func (w *RemoteClusterTunnelManager) Run(ctx context.Context) {
	w.mu.Lock()
	ctx, w.stopRun = context.WithCancel(ctx)
	w.mu.Unlock()

	for {
		if err := w.runWatchLoop(ctx); err != nil && ctx.Err() == nil {
			w.cfg.Logger.WarnContext(ctx, "Reverse tunnel watch loop failed, will retry", "error", err)
		}

		retryAfterChan := w.retry.After()
	Inner:
		for {
			select {
			case <-ctx.Done():
				w.cfg.Logger.DebugContext(ctx, "Run completed")
				return
			case <-retryAfterChan:
				w.retry.Inc()
				break Inner
			case <-time.After(defaults.ResyncInterval):
				// Attempt a sync even if the watch loop failed so that pools are updated
				// when the auth cache is unhealthy.
				if err := w.Sync(ctx); err != nil {
					w.cfg.Logger.WarnContext(ctx, "Failed to sync reverse tunnels", "error", err)
				}
			}
		}
	}
}

func (w *RemoteClusterTunnelManager) runWatchLoop(ctx context.Context) error {
	watcher, err := w.cfg.AccessPoint.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{Kind: types.KindReverseTunnel}},
	})
	if err != nil {
		// Attempt a sync even if the watcher fails so that pools are updated
		// when the auth cache is unhealthy.
		if err := w.Sync(ctx); err != nil {
			w.cfg.Logger.WarnContext(ctx, "Failed to sync reverse tunnels", "error", err)
		}

		return trace.Wrap(err, "failed to create reverse tunnel watcher")
	}
	defer watcher.Close()

	select {
	case <-ctx.Done():
		return nil
	case <-watcher.Done():
		return trace.Wrap(watcher.Error(), "watcher closed")
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected first event to be OpInit, got %s", event.Type)
		}
	case <-time.After(defaults.ResyncInterval):
		// Attempt a sync if the watcher fails to produce a timely event
		// so that pools are updated even if the auth cache is unhealthy.
		if err := w.Sync(ctx); err != nil {
			w.cfg.Logger.WarnContext(ctx, "Failed to sync reverse tunnels", "error", err)
		}

		return trace.LimitExceeded("time out waiting for OpInit event")
	}

	if err := w.Sync(ctx); err != nil {
		w.cfg.Logger.WarnContext(ctx, "Failed to sync reverse tunnels", "error", err)
		return trace.Wrap(err)
	}

	w.retry.Reset()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-watcher.Done():
			return trace.Wrap(watcher.Error(), "watcher closed")
		case event := <-watcher.Events():
			switch event.Type {
			case types.OpPut:
				if err := w.handlePut(ctx, event.Resource); err != nil {
					w.cfg.Logger.WarnContext(ctx, "failed to handle put event", "error", err)
				}
			case types.OpDelete:
				if err := w.handleDelete(event.Resource.GetName()); err != nil {
					w.cfg.Logger.WarnContext(ctx, "failed to handle delete event", "error", err)
				}
			default:
				w.cfg.Logger.DebugContext(ctx, "ignoring unexpected event", "event", event.Type)
			}
		}
	}
}

func (w *RemoteClusterTunnelManager) handleDelete(cluster string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for k, pool := range w.pools {
		if pool.Cluster != cluster {
			continue
		}

		pool.Stop()
		trustedClustersStats.DeleteLabelValues(pool.Cluster)
		delete(w.pools, k)
	}

	return nil
}

func (w *RemoteClusterTunnelManager) handlePut(ctx context.Context, r types.Resource) error {
	tunnel, ok := r.(types.ReverseTunnel)
	if !ok {
		return trace.BadParameter("expected reverse tunnel in event got %T", r)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	wantTunnels := make(map[remoteClusterKey]bool, len(tunnel.GetDialAddrs()))
	for _, addr := range tunnel.GetDialAddrs() {
		wantTunnels[remoteClusterKey{cluster: tunnel.GetClusterName(), addr: addr}] = true
	}

	// Delete pools associated with the tunnel that are no longer needed.
	for k, pool := range w.pools {
		if pool.Cluster != tunnel.GetClusterName() || wantTunnels[k] {
			continue
		}
		pool.Stop()
		trustedClustersStats.DeleteLabelValues(pool.Cluster)
		delete(w.pools, k)
	}

	var errs []error
	for _, addr := range tunnel.GetDialAddrs() {
		k := remoteClusterKey{cluster: tunnel.GetClusterName(), addr: addr}
		if _, ok := w.pools[k]; ok {
			continue
		}

		trustedClustersStats.WithLabelValues(k.cluster).Set(0)
		pool, err := w.newAgentPool(ctx, w.cfg, k.cluster, k.addr)
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}
		w.pools[k] = pool
	}

	return trace.NewAggregate(errs...)
}

// Sync does a one-time sync of trusted clusters with running agent pools.
// Non-test code should use Run() instead.
func (w *RemoteClusterTunnelManager) Sync(ctx context.Context) error {
	// Fetch desired reverse tunnels and convert them to a set of
	// remoteClusterKeys.
	wantClusters := make(map[remoteClusterKey]bool)
	for tun, err := range clientutils.Resources(ctx, w.cfg.AccessPoint.ListReverseTunnels) {
		if err != nil {
			return trace.Wrap(err)
		}

		for _, addr := range tun.GetDialAddrs() {
			wantClusters[remoteClusterKey{cluster: tun.GetClusterName(), addr: addr}] = true
		}
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Delete pools that are no longer needed.
	for k, pool := range w.pools {
		if wantClusters[k] {
			continue
		}
		pool.Stop()
		trustedClustersStats.DeleteLabelValues(pool.Cluster)
		delete(w.pools, k)
	}

	// Start pools that were added since last sync.
	var errs []error
	for k := range wantClusters {
		if _, ok := w.pools[k]; ok {
			continue
		}

		trustedClustersStats.WithLabelValues(k.cluster).Set(0)
		pool, err := w.newAgentPool(ctx, w.cfg, k.cluster, k.addr)
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}
		w.pools[k] = pool
	}
	return trace.NewAggregate(errs...)
}

func realNewAgentPool(ctx context.Context, cfg RemoteClusterTunnelManagerConfig, cluster, addr string) (*AgentPool, error) {
	pool, err := NewAgentPool(ctx, AgentPoolConfig{
		// Configs for our cluster.
		Client:              cfg.AuthClient,
		AccessPoint:         cfg.AccessPoint,
		AuthMethods:         cfg.AuthMethods,
		HostUUID:            cfg.HostUUID,
		LocalCluster:        cfg.LocalCluster,
		KubeDialAddr:        cfg.KubeDialAddr,
		ReverseTunnelServer: cfg.ReverseTunnelServer,
		FIPS:                cfg.FIPS,
		LocalAuthAddresses:  cfg.LocalAuthAddresses,
		// RemoteClusterManager only runs on proxies.
		Component: teleport.ComponentProxy,

		// Configs for remote cluster.
		Cluster:         cluster,
		Resolver:        reversetunnelclient.StaticResolver(addr, types.ProxyListenerMode_Separate),
		IsRemoteCluster: true,
		PROXYSigner:     cfg.PROXYSigner,

		StaleConnTimeoutDisabled: IsAgentStaleConnTimeoutDisabledByEnv(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed creating reverse tunnel pool for remote cluster %q at address %q: %v", cluster, addr, err)
	}

	if err := pool.Start(); err != nil {
		cfg.Logger.ErrorContext(ctx, "Failed to start agent pool", "error", err)
	}

	return pool, nil
}

// Counts returns the number of tunnels for each remote cluster.
func (w *RemoteClusterTunnelManager) Counts() map[string]int {
	w.mu.Lock()
	defer w.mu.Unlock()
	counts := make(map[string]int, len(w.pools))
	for n, p := range w.pools {
		counts[n.cluster] += p.Count()
	}
	return counts
}
