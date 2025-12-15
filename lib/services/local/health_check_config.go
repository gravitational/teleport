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

package local

import (
	"context"
	"iter"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	iterstream "github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const healthCheckConfigPrefix = "health_check_config"

// HealthCheckConfigService manages [healthcheckconfigv1.HealthCheckConfig] resources in
// the backend.
type HealthCheckConfigService struct {
	svc *generic.ServiceWrapper[*healthcheckconfigv1.HealthCheckConfig]
}

// NewHealthCheckConfigService creates a new HealthCheckConfigService.
func NewHealthCheckConfigService(b backend.Backend) (*HealthCheckConfigService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*healthcheckconfigv1.HealthCheckConfig]{
			Backend:       b,
			ResourceKind:  types.KindHealthCheckConfig,
			BackendPrefix: backend.NewKey(healthCheckConfigPrefix),
			MarshalFunc:   services.MarshalHealthCheckConfig,
			UnmarshalFunc: services.UnmarshalHealthCheckConfig,
			ValidateFunc:  services.ValidateHealthCheckConfig,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &HealthCheckConfigService{
		svc: service,
	}, nil
}

func (*HealthCheckConfigService) hasVirtualResource(name string) bool {
	switch name {
	case teleport.VirtualDefaultHealthCheckConfigDBName,
		teleport.VirtualDefaultHealthCheckConfigKubeName:
		return true
	default:
		return false
	}
}

func (*HealthCheckConfigService) getVirtualResource(name string) *healthcheckconfigv1.HealthCheckConfig {
	switch name {
	case teleport.VirtualDefaultHealthCheckConfigDBName:
		return services.VirtualDefaultHealthCheckConfigDB()
	case teleport.VirtualDefaultHealthCheckConfigKubeName:
		return services.VirtualDefaultHealthCheckConfigKube()
	}
	return nil
}

func (*HealthCheckConfigService) rangeVirtualResources(start string) iter.Seq2[*healthcheckconfigv1.HealthCheckConfig, error] {
	return func(yield func(*healthcheckconfigv1.HealthCheckConfig, error) bool) {
		switch {
		case start <= teleport.VirtualDefaultHealthCheckConfigDBName:
			if !yield(services.VirtualDefaultHealthCheckConfigDB(), nil) {
				return
			}
			fallthrough
		case start <= teleport.VirtualDefaultHealthCheckConfigKubeName:
			if !yield(services.VirtualDefaultHealthCheckConfigKube(), nil) {
				return
			}
		}
	}
}

// CreateHealthCheckConfig creates a new HealthCheckConfig resource.
func (s *HealthCheckConfigService) CreateHealthCheckConfig(ctx context.Context, config *healthcheckconfigv1.HealthCheckConfig) (*healthcheckconfigv1.HealthCheckConfig, error) {
	// we don't need to check if config refers to a potentially virtual resource
	// because creating on top of a virtual resource is very convenient so it's
	// a break from the semantics of Create that we want to allow
	created, err := s.svc.CreateResource(ctx, config)
	return created, trace.Wrap(err)
}

// GetHealthCheckConfig returns the specified HealthCheckConfig resource.
// A virtual resource, if requested, is always returned even if it doesn't exist in the backend.
func (s *HealthCheckConfigService) GetHealthCheckConfig(ctx context.Context, name string) (*healthcheckconfigv1.HealthCheckConfig, error) {
	item, err := s.svc.GetResource(ctx, name)
	if err != nil {
		if trace.IsNotFound(err) {
			if virtualResource := s.getVirtualResource(name); virtualResource != nil {
				return virtualResource, nil
			}
		}
		return nil, trace.Wrap(err)
	}
	return item, nil
}

// ListHealthCheckConfigs returns a paginated list of HealthCheckConfig resources.
// Virtual resources are always returned even if they don't exist in the backend.
func (s *HealthCheckConfigService) ListHealthCheckConfigs(ctx context.Context, pageSize int, pageToken string) ([]*healthcheckconfigv1.HealthCheckConfig, string, error) {
	items, nextPageToken, err := generic.CollectPageAndCursor(
		iterstream.MergeStreamsWithPriority(
			s.svc.Resources(ctx, pageToken, ""),
			s.rangeVirtualResources(pageToken),
			func(a, b *healthcheckconfigv1.HealthCheckConfig) int {
				return strings.Compare(a.GetMetadata().GetName(), b.GetMetadata().GetName())
			},
		),
		pageSize,
		func(v *healthcheckconfigv1.HealthCheckConfig) string {
			return v.GetMetadata().GetName()
		},
	)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return items, nextPageToken, nil
}

// UpdateHealthCheckConfig updates an existing HealthCheckConfig resource.
func (s *HealthCheckConfigService) UpdateHealthCheckConfig(ctx context.Context, config *healthcheckconfigv1.HealthCheckConfig) (*healthcheckconfigv1.HealthCheckConfig, error) {
	if virtualResource := s.getVirtualResource(config.GetMetadata().GetName()); virtualResource != nil && config.GetMetadata().GetRevision() == virtualResource.GetMetadata().GetRevision() {
		// a (successful) conditional update on a virtual resource is a create
		// in storage; no real storage item is going to have the same revision
		// as the virtual resource, so this must be a conditional update on the
		// virtual resource that we know of
		created, err := s.svc.CreateResource(ctx, config)
		if err != nil {
			if trace.IsAlreadyExists(err) {
				return nil, trace.Wrap(backend.ErrIncorrectRevision)
			}
			return nil, trace.Wrap(err)
		}
		return created, nil
	}
	// if this was intended to be a conditional update on a virtual resource it
	// was not a successful one, but we can let the storage deal with it
	updated, err := s.svc.ConditionalUpdateResource(ctx, config)
	return updated, trace.Wrap(err)
}

// UpsertHealthCheckConfig upserts an existing HealthCheckConfig resource.
func (s *HealthCheckConfigService) UpsertHealthCheckConfig(ctx context.Context, config *healthcheckconfigv1.HealthCheckConfig) (*healthcheckconfigv1.HealthCheckConfig, error) {
	upserted, err := s.svc.UpsertResource(ctx, config)
	return upserted, trace.Wrap(err)
}

// DeleteHealthCheckConfig removes the specified HealthCheckConfig resource.
func (s *HealthCheckConfigService) DeleteHealthCheckConfig(ctx context.Context, name string) error {
	err := s.svc.DeleteResource(ctx, name)
	if trace.IsNotFound(err) && s.hasVirtualResource(name) {
		// we want to allow deleting virtual resources as a noop even if it's a
		// little break from the standard semantics of Delete
		return nil
	}
	return trace.Wrap(err)
}

// DeleteAllHealthCheckConfigs removes all HealthCheckConfig resources.
func (s *HealthCheckConfigService) DeleteAllHealthCheckConfigs(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}

func newHealthCheckConfigParser() resourceParser {
	return healthCheckConfigParser{}
}

type healthCheckConfigParser struct{}

func (healthCheckConfigParser) prefixes() []backend.Key {
	return []backend.Key{
		backend.ExactKey(healthCheckConfigPrefix),
	}
}

func (healthCheckConfigParser) match(key backend.Key) bool {
	return strings.HasPrefix(key.String(), backend.SeparatorString+healthCheckConfigPrefix+backend.SeparatorString)
}

func (healthCheckConfigParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		key := event.Item.Key.String()
		name, found := strings.CutPrefix(key, backend.SeparatorString+healthCheckConfigPrefix+backend.SeparatorString)
		if !found {
			return nil, trace.NotFound("failed parsing "+types.KindHealthCheckConfig+" key %+q", key)
		}

		if virtualResource := (*HealthCheckConfigService)(nil).getVirtualResource(name); virtualResource != nil {
			return nil, parseEventOverrideError{{
				Type:     types.OpPut,
				Resource: types.Resource153ToLegacy(virtualResource),
			}}
		}

		return &types.ResourceHeader{
			Kind:    types.KindHealthCheckConfig,
			Version: types.V1,
			Metadata: types.Metadata{
				Name:      name,
				Namespace: apidefaults.Namespace,
			},
		}, nil
	case types.OpPut:
		resource, err := services.UnmarshalHealthCheckConfig(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(resource), nil
	default:
		return nil, trace.BadParameter("event %s is not supported", event.Type)
	}
}

func itemFromHealthCheckConfig(cfg *healthcheckconfigv1.HealthCheckConfig) (*backend.Item, error) {
	if err := services.ValidateHealthCheckConfig(cfg); err != nil {
		return nil, trace.Wrap(err)
	}
	rev, err := types.GetRevision(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalHealthCheckConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	expires, err := types.GetExpiry(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:      backend.NewKey(healthCheckConfigPrefix, cfg.GetMetadata().GetName()),
		Value:    value,
		Expires:  expires,
		Revision: rev,
	}
	return item, nil
}
