/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package healthcheck

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// Manager manages health checks for registered resource targets.
type Manager interface {
	// Start starts the health check manager.
	Start(ctx context.Context) error
	// AddTarget adds a new target health checker and starts the health checker.
	AddTarget(target Target) error
	// RemoveTarget stops a given resource's health checker and removes it from
	// the manager's monitored resources.
	RemoveTarget(r types.ResourceWithLabels) error
	// GetTargetHealth returns the health of a given resource.
	GetTargetHealth(r types.ResourceWithLabels) (*types.TargetHealth, error)
	// Close closes the health check manager and stops all health checkers.
	Close() error
}

// ManagerConfig is the configuration options for [Manager].
type ManagerConfig struct {
	// Clock is optional and can be used to control time in tests.
	Clock clockwork.Clock
	// Component is a component used in logs.
	Component string
	// Events is used to create new event watchers.
	Events types.Events
	// HealthCheckConfigReader can list or get health check config resources.
	HealthCheckConfigReader services.HealthCheckConfigReader
}

// checkAndSetDefaults checks the manager config for errors and applies defaults.
func (c *ManagerConfig) checkAndSetDefaults() error {
	if c.Component == "" {
		return trace.BadParameter("missing Component")
	}
	if c.Events == nil {
		return trace.BadParameter("missing Events")
	}
	if c.HealthCheckConfigReader == nil {
		return trace.BadParameter("missing HealthCheckConfigReader")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewManager creates a new unstarted health check [Manager].
func NewManager(ctx context.Context, cfg ManagerConfig) (Manager, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	mgr := &manager{
		closeContext: ctx,
		closeFn:      cancel,
		cfg:          cfg,
		logger:       slog.With(teleport.ComponentKey, cfg.Component),
		workers:      make(map[resourceKey]*worker),
	}
	return mgr, nil
}

// manager implements [Manager].
type manager struct {
	cfg          ManagerConfig
	closeContext context.Context
	closeFn      context.CancelFunc
	logger       *slog.Logger

	// mu guards concurrent access to the monitored health check configs and
	// target resource workers.
	mu      sync.RWMutex
	configs []*healthCheckConfig
	workers map[resourceKey]*worker
}

// newResourceKey creates a new key for a given resource.
func newResourceKey(r types.ResourceWithLabels) resourceKey {
	return resourceKey{
		name: r.GetName(),
		kind: r.GetKind(),
	}
}

// resourceKey is a map key for target resource workers.
type resourceKey struct {
	// name is the target resource name.
	name string
	// kind is the target resource kind.
	kind string
}

func (r resourceKey) String() string {
	return fmt.Sprintf("name=%s, kind=%s", r.name, r.kind)
}

// Start starts the health check manager.
func (m *manager) Start(ctx context.Context) error {
	if err := m.startConfigWatcher(ctx); err != nil {
		return trace.Wrap(err, "failed to start health check config watcher")
	}
	m.startWorkerUpdater(ctx)
	return nil
}

// supportedTargetKinds is a list of resource kinds that support health checks.
var supportedTargetKinds = []string{
	types.KindDatabase,
}

// AddTarget adds a new target health checker and starts the health checker.
func (m *manager) AddTarget(target Target) error {
	if err := target.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	resource := target.GetResource()
	if !slices.Contains(supportedTargetKinds, resource.GetKind()) {
		return trace.BadParameter("health check target resource kind %q is not supported", resource.GetKind())
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	key := newResourceKey(resource)
	if _, ok := m.workers[key]; ok {
		return trace.AlreadyExists("target health checker %q already exists", key)
	}
	worker, err := newWorker(m.closeContext, workerConfig{
		Clock:          m.cfg.Clock,
		HealthCheckCfg: m.getConfigLocked(m.closeContext, resource),
		Log: m.logger.With(
			"target_name", resource.GetName(),
			"target_kind", resource.GetKind(),
			"target_origin", resource.Origin(),
		),
		Target: target,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	m.workers[key] = worker
	return nil
}

// RemoveTarget stops a given resource's health checker and removes it from
// the manager's monitored resources.
func (m *manager) RemoveTarget(r types.ResourceWithLabels) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := newResourceKey(r)
	worker, ok := m.workers[key]
	if !ok {
		return trace.NotFound("health checker %q not found", key)
	}
	delete(m.workers, key)
	if err := worker.Close(); err != nil {
		m.logger.DebugContext(context.Background(),
			"Health checker failed to close",
			"error", err,
		)
	}
	return nil
}

// GetTargetHealth returns the health of a given resource.
func (m *manager) GetTargetHealth(r types.ResourceWithLabels) (*types.TargetHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := newResourceKey(r)
	worker, ok := m.workers[key]
	if !ok {
		return nil, trace.NotFound("health checker %q not found", key)
	}
	return worker.GetTargetHealth(), nil
}

// Close closes the health check manager and stops all health checkers.
func (m *manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeFn()
	var errors []error
	for _, w := range m.workers {
		errors = append(errors, w.Close())
	}
	clear(m.workers)
	clear(m.configs)
	return trace.NewAggregate(errors...)
}

// startConfigWatcher starts a watcher for health check config resources.
func (m *manager) startConfigWatcher(ctx context.Context) error {
	watcher, err := services.NewHealthCheckConfigWatcher(ctx,
		services.HealthCheckConfigWatcherConfig{
			Reader: m.cfg.HealthCheckConfigReader,
			ResourceWatcherConfig: services.ResourceWatcherConfig{
				Client:    m.cfg.Events,
				Clock:     m.cfg.Clock,
				Component: m.cfg.Component,
				Logger:    m.logger,
			},
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	m.logger.DebugContext(ctx, "Started health check config resource watcher")
	var initOnce sync.Once
	initCh := make(chan struct{})
	go func() {
		defer watcher.Close()
		defer m.logger.DebugContext(ctx, "Stopped health check config resource watcher")
		for {
			select {
			case configs := <-watcher.ResourcesC:
				m.updateConfigs(ctx, configs)
				initOnce.Do(func() { close(initCh) })
			case <-watcher.Done():
				return
			case <-m.closeContext.Done():
				return
			}
		}
	}()
	if err := watcher.WaitInitialization(); err != nil {
		return trace.Wrap(err)
	}
	select {
	case <-watcher.Done():
	case <-initCh:
	}
	return nil
}

// updateConfigs updates the manager's known health check configs and
// recalculates the matching config for each registered target resource.
func (m *manager) updateConfigs(ctx context.Context, cs []*healthcheckconfigv1.HealthCheckConfig) {
	// Config priority is by ascending order of name - the first config to match, wins.
	slices.SortFunc(cs, func(a, b *healthcheckconfigv1.HealthCheckConfig) int {
		return cmp.Compare(a.GetMetadata().GetName(), b.GetMetadata().GetName())
	})
	configs := make([]*healthCheckConfig, 0, len(cs))
	for _, c := range cs {
		configs = append(configs, newHealthCheckConfig(c))
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs = configs
	m.updateWorkersLocked(ctx)
}

// startWorkerUpdater periodically evaluates and updates the matching health
// check config for each worker.
// The config for a worker may change if its target resource labels change
// dynamically.
func (m *manager) startWorkerUpdater(ctx context.Context) {
	go func() {
		updateInterval := interval.New(interval.Config{
			Duration: time.Minute,
			Jitter:   retryutils.SeventhJitter,
			Clock:    m.cfg.Clock,
		})
		defer updateInterval.Stop()
		for {
			select {
			case <-updateInterval.Next():
				m.mu.Lock()
				m.updateWorkersLocked(ctx)
				m.mu.Unlock()
			case <-ctx.Done():
				return
			case <-m.closeContext.Done():
				return
			}
		}
	}()
}

// updateWorkersLocked evaluates and updates the matching health check config
// for each worker.
func (m *manager) updateWorkersLocked(ctx context.Context) {
	for _, w := range m.workers {
		newCfg := m.getConfigLocked(ctx, w.GetTargetResource())
		w.UpdateHealthCheckConfig(newCfg)
	}
}

// getConfigLocked gets a matching config for the the given resource or returns
// nil if no config matches.
func (m *manager) getConfigLocked(ctx context.Context, r types.ResourceWithLabels) *healthCheckConfig {
	for _, cfg := range m.configs {
		matched, _, err := services.CheckLabelsMatch(
			types.Allow,
			cfg.getLabelMatchers(r.GetKind()),
			nil, // userTraits
			r,
			false, // debug
		)
		if err != nil {
			m.logger.WarnContext(ctx, "Health check config failed to match",
				"health_check_config", cfg.name,
				"error", err,
			)
			continue
		}
		if matched {
			return cfg
		}
	}
	return nil
}
