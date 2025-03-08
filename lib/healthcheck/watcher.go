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
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func (s *service) startConfigWatcher(ctx context.Context) error {
	watcher, err := services.NewHealthCheckConfigWatcher(ctx,
		services.HealthCheckConfigWatcherConfig{
			Reader: s.cfg.Reader,
			ResourceWatcherConfig: services.ResourceWatcherConfig{
				Client:    s.cfg.Events,
				Component: teleport.ComponentDatabase,
				Logger:    s.cfg.Logger,
			},
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	s.watcher = watcher
	s.cfg.Logger.DebugContext(ctx, "Started health check config resource watcher")
	go func() {
		defer s.cfg.Logger.DebugContext(ctx, "Stopped health check config resource watcher")
		defer watcher.Close()
		for {
			select {
			case configs := <-watcher.ResourcesC:
				s.updateConfigs(configs)
			case <-watcher.Done():
				return
			}
		}
	}()
	return nil
}

func (s *service) updateConfigs(cs []*healthcheckconfigv1.HealthCheckConfig) {
	// Config priority is by ascending order of name - the first config to match "wins".
	slices.SortFunc(cs, func(a, b *healthcheckconfigv1.HealthCheckConfig) int {
		return cmp.Compare(a.GetMetadata().GetName(), b.GetMetadata().GetName())
	})
	configs := make([]healthCheckConfig, 0, len(cs))
	for _, c := range cs {
		configs = append(configs, s.convertConfigProto(c))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.configs = configs
	for _, c := range s.checkers {
		c.UpdateHealthCheckConfig(s.configs)
	}
}

// convertConfigProto converts a health check config resource protobuf message
// into a [healthCheckConfig].
func (s *service) convertConfigProto(cfg *healthcheckconfigv1.HealthCheckConfig) healthCheckConfig {
	spec := cfg.GetSpec()
	return healthCheckConfig{
		name:               cfg.Metadata.GetName(),
		timeout:            cmp.Or(spec.GetTimeout().AsDuration(), defaults.HealthCheckTimeout),
		interval:           cmp.Or(spec.GetInterval().AsDuration(), defaults.HealthCheckInterval),
		healthyThreshold:   int(cmp.Or(spec.GetHealthyThreshold(), defaults.HealthCheckHealthyThreshold)),
		unhealthyThreshold: int(cmp.Or(spec.GetUnhealthyThreshold(), defaults.HealthCheckUnhealthyThreshold)),
		// we only support plain TCP health checks currently, but eventually we
		// may add support for other protocols such as TLS or HTTP.
		protocol: types.TargetHealthProtocolTCP,
		matcher:  s.newLabelMatcherFn(spec.GetMatch()),
	}
}
