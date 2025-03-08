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
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
)

// Service manages health checks for registered targets.
type Service interface {
	// Start starts the health check service.
	Start(ctx context.Context) error
	// AddTarget adds a new target health checker and starts the health checker.
	AddTarget(ctx context.Context, r types.ResourceWithLabels, resolverFn EndpointsResolverFunc) error
	// RemoveTarget stops a given resource's health checker and removes it from
	// the service's monitored resources.
	RemoveTarget(r types.ResourceWithLabels) error
	// GetTargetHealth returns the health of a given resource.
	GetTargetHealth(r types.ResourceWithLabels) (*types.TargetHealth, error)
	// Close closes the service. All health checkers are stopped and removed.
	Close() error
}

// EndpointsResolverFunc is callback func that returns endpoints for a target.
type EndpointsResolverFunc func(ctx context.Context) ([]string, error)

// ServiceConfig is the configuration options for [Service].
type ServiceConfig struct {
	// Clock is optional and can be used to control time in tests.
	Clock clockwork.Clock
	// Events is used to create new event watchers.
	Events types.Events
	// Logger is the service's logger.
	Logger *slog.Logger
	// Reader can list or get health check config resources.
	Reader services.HealthCheckConfigReader
}

func (c *ServiceConfig) checkAndSetDefaults() error {
	if c.Events == nil {
		return trace.BadParameter("missing Events")
	}
	if c.Logger == nil {
		return trace.BadParameter("missing Logger")
	}
	if c.Reader == nil {
		return trace.BadParameter("missing Reader")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewDBHealthService returns a new [Service] configured for monitoring
// the health of database resources.
func NewDBHealthService(ctx context.Context, cfg ServiceConfig) (Service, error) {
	svc, err := newHealthService(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	svc.newLabelMatcherFn = newDBLabelMatcher
	return svc, nil
}

func newHealthService(cfg ServiceConfig) (*service, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	svc := &service{
		cfg:      cfg,
		checkers: make(map[checkerKey]healthChecker),
	}
	return svc, nil
}

type service struct {
	cfg               ServiceConfig
	newLabelMatcherFn newLabelMatcherFunc
	running           atomic.Bool

	mu       sync.Mutex
	configs  []healthCheckConfig
	checkers map[checkerKey]healthChecker
	watcher  *services.HealthCheckConfigWatcher
}

// Start starts the health check service.
func (s *service) Start(ctx context.Context) error {
	if !s.running.CompareAndSwap(false, true) {
		return trace.AlreadyExists("Health check service is already running")
	}
	if err := s.startConfigWatcher(ctx); err != nil {
		return trace.Wrap(err, "failed to start health check config watcher")
	}
	return nil
}

// AddTarget adds a new target health checker and starts the health checker.
func (s *service) AddTarget(ctx context.Context, r types.ResourceWithLabels, resolverFn EndpointsResolverFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := newCheckerKey(r)
	if _, ok := s.checkers[key]; ok {
		return trace.AlreadyExists("target health checker %q already exists", key)
	}
	checker, err := newHealthChecker(ctx, healthCheckerConfig{
		clock:      s.cfg.Clock,
		resource:   r,
		resolverFn: resolverFn,
		log:        s.cfg.Logger,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	s.checkers[key] = checker
	checker.UpdateHealthCheckConfig(s.configs)
	return nil
}

// RemoveTarget stops a given resource's health checker and removes it from
// the service's monitored resources.
func (s *service) RemoveTarget(r types.ResourceWithLabels) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := newCheckerKey(r)
	checker, ok := s.checkers[key]
	if !ok {
		return trace.NotFound("health checker %q not found", key)
	}
	delete(s.checkers, key)
	if err := checker.Close(); err != nil {
		s.cfg.Logger.Debug("Health checker failed to close",
			"error", err,
		)
	}
	return nil
}

// GetTargetHealth returns the health of a given resource.
func (s *service) GetTargetHealth(r types.ResourceWithLabels) (*types.TargetHealth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := newCheckerKey(r)
	checker, ok := s.checkers[key]
	if !ok {
		return nil, trace.NotFound("health checker %q not found", key)
	}
	return checker.GetTargetHealth(), nil
}

// Close closes the service. All health checkers are stopped and removed.
func (s *service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watcher.Close()
	var errors []error
	for _, c := range s.checkers {
		errors = append(errors, c.Close())
	}
	clear(s.checkers)
	s.running.Store(false)
	return trace.NewAggregate(errors...)
}

func newCheckerKey(r types.ResourceWithLabels) checkerKey {
	return checkerKey{
		name:   r.GetName(),
		origin: r.Origin(),
	}
}

type checkerKey struct {
	name   string
	origin string
}

func (c checkerKey) String() string {
	return fmt.Sprintf("name=%s, origin=%s", c.name, c.origin)
}

// newLabelMatcherFunc creates a new [types.LabelMatchers] from a health check
// config resource's matcher.
type newLabelMatcherFunc func(*healthcheckconfigv1.Matcher) types.LabelMatchers

// newDBLabelMatcher is a [newLabelMatcherFunc].
func newDBLabelMatcher(matcher *healthcheckconfigv1.Matcher) types.LabelMatchers {
	return newLabelMatchers(matcher.GetDbLabelsExpression(), matcher.GetDbLabels())
}

// newLabelMatchers creates a new [types.LabelMatchers] from an expression and
// r153 labels.
func newLabelMatchers(expr string, labels []*labelv1.Label) types.LabelMatchers {
	out := types.LabelMatchers{
		Expression: expr,
		Labels:     make(types.Labels),
	}
	for k, vs := range label.ToMap(labels) {
		out.Labels[k] = apiutils.Strings(vs)
	}
	return out
}
