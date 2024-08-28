/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package services

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// GitServerWatcherConfig is a GitServerWatcher configuration.
type GitServerWatcherConfig struct {
	ResourceWatcherConfig
	GitServersGetter
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *GitServerWatcherConfig) CheckAndSetDefaults() error {
	if err := cfg.ResourceWatcherConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.GitServersGetter == nil {
		getter, ok := cfg.Client.(GitServersGetter)
		if !ok {
			return trace.BadParameter("missing parameter GitServersGetter and Client not usable as GitServersGetter")
		}
		cfg.GitServersGetter = getter
	}
	return nil
}

// TODO refactor
type GitServerWatcher struct {
	*resourceWatcher
	*gitServerCollector
}

// gitServerCollector accompanies resourceWatcher when monitoring gitServers.
type gitServerCollector struct {
	GitServerWatcherConfig

	// initializationC is used to check whether the initial sync has completed
	initializationC chan struct{}
	once            sync.Once

	cache *utils.FnCache

	rw sync.RWMutex
	// current holds a map of the currently known gitServers keyed by server name
	current map[string]types.Server
	stale   bool
}

// NewGitServerWatcher returns a new instance of GitServerWatcher.
func NewGitServerWatcher(ctx context.Context, cfg GitServerWatcherConfig) (*GitServerWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		Context: ctx,
		TTL:     3 * time.Second,
		Clock:   cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	collector := &gitServerCollector{
		GitServerWatcherConfig: cfg,
		current:                map[string]types.Server{},
		initializationC:        make(chan struct{}),
		cache:                  cache,
		stale:                  true,
	}

	watcher, err := newResourceWatcher(ctx, collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &GitServerWatcher{resourceWatcher: watcher, gitServerCollector: collector}, nil
}

// GitServer is a readonly subset of the types.Server interface which
// users may filter by in GetGitServers.
type GitServer interface {
	types.ResourceWithLabels
	GetTeleportVersion() string
	GetGitHub() *types.GitHubServerMetadata
}

// GetGitServers allows callers to retrieve a subset of gitServers that match the filter provided. The
// returned servers are a copy and can be safely modified. It is intentionally hard to retrieve
// the full set of gitServers to reduce the number of copies needed since the number of gitServers can get
// quite large and doing so can be expensive.
func (n *gitServerCollector) GetGitServers(ctx context.Context, fn func(n GitServer) bool) []types.Server {
	// Attempt to freshen our data first.
	n.refreshStaleGitServers(ctx)

	n.rw.RLock()
	defer n.rw.RUnlock()

	var matched []types.Server
	for _, server := range n.current {
		if fn(server) {
			matched = append(matched, server.DeepCopy())
		}
	}

	return matched
}

// GetGitServer allows callers to retrieve a gitServer based on its name. The
// returned server are a copy and can be safely modified.
func (n *gitServerCollector) GetGitServer(ctx context.Context, name string) (types.Server, error) {
	// Attempt to freshen our data first.
	n.refreshStaleGitServers(ctx)

	n.rw.RLock()
	defer n.rw.RUnlock()

	server, found := n.current[name]
	if !found {
		return nil, trace.NotFound("server does not exist")
	}
	return server.DeepCopy(), nil
}

// refreshStaleGitServers attempts to reload gitServers from the GitServerGetter if
// the collecter is stale. This ensures that no matter the health of
// the collecter callers will be returned the most up to date gitServer
// set as possible.
func (n *gitServerCollector) refreshStaleGitServers(ctx context.Context) error {
	n.rw.RLock()
	if !n.stale {
		n.rw.RUnlock()
		return nil
	}
	n.rw.RUnlock()

	_, err := utils.FnCacheGet(ctx, n.cache, "gitServers", func(ctx context.Context) (any, error) {
		current, err := n.getGitServers(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		n.rw.Lock()
		defer n.rw.Unlock()

		// There is a chance that the watcher reinitialized while
		// getting gitServers happened above. Check if we are still stale
		// now that the lock is held to ensure that the refresh is
		// still necessary.
		if !n.stale {
			return nil, nil
		}

		n.current = current
		return nil, trace.Wrap(err)
	})

	return trace.Wrap(err)
}

func (n *gitServerCollector) GitServerCount() int {
	n.rw.RLock()
	defer n.rw.RUnlock()
	return len(n.current)
}

// resourceKinds specifies the resource kind to watch.
func (n *gitServerCollector) resourceKinds() []types.WatchKind {
	return []types.WatchKind{{Kind: types.KindGitServer}}
}

// getResourcesAndUpdateCurrent is called when the resources should be
// (re-)fetched directly.
func (n *gitServerCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	newCurrent, err := n.getGitServers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer n.defineCollectorAsInitialized()

	if len(newCurrent) == 0 {
		return nil
	}

	n.rw.Lock()
	defer n.rw.Unlock()
	n.current = newCurrent
	n.stale = false
	return nil
}

func (n *gitServerCollector) getGitServers(ctx context.Context) (map[string]types.Server, error) {
	gitServers, err := n.GitServersGetter.GetGitServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(gitServers) == 0 {
		return map[string]types.Server{}, nil
	}

	current := make(map[string]types.Server, len(gitServers))
	for _, gitServer := range gitServers {
		current[gitServer.GetName()] = gitServer
	}

	return current, nil
}

func (n *gitServerCollector) defineCollectorAsInitialized() {
	n.once.Do(func() {
		// mark watcher as initialized.
		close(n.initializationC)
	})
}

// processEventsAndUpdateCurrent is called when a watcher event is received.
func (n *gitServerCollector) processEventsAndUpdateCurrent(ctx context.Context, events []types.Event) {
	n.rw.Lock()
	defer n.rw.Unlock()

	for _, event := range events {
		if event.Resource == nil || event.Resource.GetKind() != types.KindGitServer {
			n.Log.Warningf("Unexpected event: %v.", event)
			continue
		}

		switch event.Type {
		case types.OpDelete:
			delete(n.current, event.Resource.GetName())
		case types.OpPut:
			server, ok := event.Resource.(types.Server)
			if !ok {
				n.Log.Warningf("Unexpected type %T.", event.Resource)
				continue
			}

			n.current[server.GetName()] = server
		default:
			n.Log.Warningf("Skipping unsupported event type %s.", event.Type)
		}
	}
}

func (n *gitServerCollector) initializationChan() <-chan struct{} {
	return n.initializationC
}

func (n *gitServerCollector) notifyStale() {
	n.rw.Lock()
	defer n.rw.Unlock()
	n.stale = true
}
