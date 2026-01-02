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

package metrics

import (
	"cmp"
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

// Registry is a [prometheus.Registerer] for a Teleport process that
// allows propagating additional information such as:
//   - the metric namespace (`teleport`, `teleport_bot`, `teleport_plugins`)
//   - an optional subsystem
//
// This should be passed anywhere that needs to register a metric.
type Registry struct {
	prometheus.Registerer

	namespace string
	subsystem string
}

// Namespace returns the namespace that should be used by metrics registered
// in this Registry. Common namespaces are "teleport", "tbot", and
// "teleport_plugins".
func (r *Registry) Namespace() string {
	return r.namespace
}

// Subsystem is the subsystem base that should be used by metrics registered in
// this Registry. Subsystem parts can be added with WrapWithSubsystem.
func (r *Registry) Subsystem() string {
	return r.subsystem
}

// Wrap wraps a Registry by adding a component to its subsystem.
// This should be used before passing a registry to a sub-component.
// Example usage:
//
//	 rootReg := prometheus.NewRegistry()
//	 process.AddGatherer(rootReg)
//		reg, err := NewRegistry(rootReg, "teleport_plugins", "")
//	 go runFooService(ctx, log, reg.Wrap("foo"))
//	 go runBarService(ctx, log, reg.Wrap("bar"))
func (r *Registry) Wrap(subsystem string) *Registry {
	if r.subsystem != "" && subsystem != "" {
		subsystem = r.subsystem + "_" + subsystem
	} else {
		subsystem = cmp.Or(r.subsystem, subsystem)
	}

	newReg := &Registry{
		Registerer: r.Registerer,
		namespace:  r.namespace,
		subsystem:  subsystem,
	}
	return newReg
}

// NewRegistry creates a new Registry wrapping a prometheus registry.
// This should only be called when starting the service management routines such
// as: service.NewTeleport(), tbot.New(), or the hosted plugin manager.
// Services and sub-services should take the registry as a parameter, like they
// already do for the logger.
// Example usage:
//
//	 rootReg := prometheus.NewRegistry()
//	 process.AddGatherer(rootReg)
//		reg, err := NewRegistry(rootReg, "teleport_plugins", "")
//	 go runFooService(ctx, log, reg.Wrap("foo"))
//	 go runBarService(ctx, log, reg.Wrap("bar"))
func NewRegistry(reg prometheus.Registerer, namespace, subsystem string) (*Registry, error) {
	if reg == nil {
		return nil, errors.New("nil prometheus.Registerer (this is a bug)")
	}
	if namespace == "" {
		return nil, errors.New("namespace is required (this is a bug)")
	}
	return &Registry{
		Registerer: reg,
		namespace:  namespace,
		subsystem:  subsystem,
	}, nil
}

// NoopRegistry returns a Registry that doesn't register metrics.
// This can be used in tests, or to provide backward compatibility when a nil
// Registry is passed.
func NoopRegistry() *Registry {
	return &Registry{
		Registerer: noopRegistry{},
		namespace:  "noop",
		subsystem:  "",
	}
}

type noopRegistry struct{}

// Register implements [prometheus.Registerer].
func (b noopRegistry) Register(collector prometheus.Collector) error {
	return nil
}

// MustRegister implements [prometheus.Registerer].
func (b noopRegistry) MustRegister(collector ...prometheus.Collector) {
}

// Unregister implements [prometheus.Registerer].
func (b noopRegistry) Unregister(collector prometheus.Collector) bool {
	return true
}
