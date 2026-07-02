// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package local

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1/expression"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/utils/typical"
)

const (
	botInstancePrefix = "bot_instance"
	// scopedPrefix is the top-level backend prefix under which instances of
	// scoped bots are namespaced by their bot's scope, keyed as
	// scoped/bot_instance/<encoded scope>/<bot name>/<instance id>. Instances
	// of unscoped bots remain keyed as bot_instance/<bot name>/<instance id>.
	scopedPrefix = "scoped"
)

// BotInstanceService exposes backend functionality for storing bot instances.
type BotInstanceService struct {
	service *generic.ScopeAwareServiceWrapper[*machineidv1.BotInstance]

	clock clockwork.Clock
}

// NewBotInstanceService creates a new BotInstanceService with the given backend.
func NewBotInstanceService(b backend.Backend, clock clockwork.Clock) (*BotInstanceService, error) {
	service, err := generic.NewScopeAwareServiceWrapper(
		generic.ScopeAwareServiceWrapperConfig[*machineidv1.BotInstance]{
			Backend:               b,
			ResourceKind:          types.KindBotInstance,
			UnscopedBackendPrefix: backend.NewKey(botInstancePrefix),
			ScopedBackendPrefix:   backend.NewKey(scopedPrefix, botInstancePrefix),
			MarshalFunc:           services.MarshalBotInstance,
			UnmarshalFunc:         services.UnmarshalBotInstance,
			ValidateFunc:          services.ValidateBotInstance,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &BotInstanceService{
		service: service,
		clock:   clock,
	}, nil
}

// serviceForBot returns a single-range service addressing the instances of the
// bot identified by (botScope, botName): the bot's sub-range of the scoped key
// range when botScope is non-empty, else its sub-range of the unscoped range.
func (b *BotInstanceService) serviceForBot(botScope, botName string) (*generic.ServiceWrapper[*machineidv1.BotInstance], error) {
	service, err := b.service.WithScopePrefix(botScope)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return service.WithPrefix(botName), nil
}

// CreateBotInstance inserts a new BotInstance into the backend. It is stored
// in the key range determined by the scope set on the instance itself.
//
// Note that new BotInstances will have their .Metadata.Name overwritten by the
// instance UUID.
func (b *BotInstanceService) CreateBotInstance(ctx context.Context, instance *machineidv1.BotInstance) (*machineidv1.BotInstance, error) {
	instance.SetKind(types.KindBotInstance)
	instance.SetVersion(types.V1)

	if !instance.HasMetadata() {
		instance.SetMetadata(&headerv1.Metadata{})
	}

	instance.GetMetadata().SetName(instance.GetSpec().GetInstanceId())

	serviceWithPrefix, err := b.serviceForBot(instance.GetScope(), instance.GetSpec().GetBotName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := serviceWithPrefix.CreateResource(ctx, instance)
	return created, trace.Wrap(err)
}

// GetBotInstance retreives a specific bot instance given a bot scope, bot name
// and instance ID. The scope must be empty if the owning bot is unscoped.
func (b *BotInstanceService) GetBotInstance(ctx context.Context, botScope, botName, instanceID string) (*machineidv1.BotInstance, error) {
	serviceWithPrefix, err := b.serviceForBot(botScope, botName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	instance, err := serviceWithPrefix.GetResource(ctx, instanceID)
	return instance, trace.Wrap(err)
}

// ListBotInstances lists all matching bot instances. A bot (scope, name) and/or search terms can be optionally provided.
// If an non-empty bot name is provided, only instances for that bot will be fetched. The bot scope must be
// provided alongside the name for a scoped bot's instances; with no bot filter, instances for all bots are
// listed, unscoped bots' instances first.
// If an non-empty search term is provided, only instances with a value containing the term in supported fields are fetched.
// Supported search fields include; bot name, instance id, hostname (latest), tbot version (latest), join method (latest).
// Sorting by bot name in ascending order is supported - an error is returned for any other sort type.
func (b *BotInstanceService) ListBotInstances(ctx context.Context, pageSize int, lastKey string, options *services.ListBotInstancesRequestOptions) ([]*machineidv1.BotInstance, string, error) {
	if options.GetSortField() != "" && options.GetSortField() != "bot_name" {
		return nil, "", trace.CompareFailed("unsupported sort, only bot_name field is supported, but got %q", options.GetSortField())
	}
	if options.GetSortDesc() {
		return nil, "", trace.CompareFailed("unsupported sort, only ascending order is supported")
	}

	// Satisfied by both the scope-aware wrapper (unified listing across the
	// unscoped and scoped key ranges) and a single-range service routed by the
	// bot filter.
	var service interface {
		ListResources(ctx context.Context, pageSize int, nextToken string) ([]*machineidv1.BotInstance, string, error)
		ListResourcesWithFilter(ctx context.Context, pageSize int, nextToken string, matcher func(*machineidv1.BotInstance) bool) ([]*machineidv1.BotInstance, string, error)
	}
	if options.GetFilterBotName() == "" && options.GetFilterBotScope() == "" {
		// If no bot filter is set, return instances for all bots across both
		// the unscoped and scoped key ranges.
		service = b.service
	} else {
		routed, err := b.service.WithScopePrefix(options.GetFilterBotScope())
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		if options.GetFilterBotName() != "" {
			routed = routed.WithPrefix(options.GetFilterBotName())
		}
		service = routed
	}

	var exp typical.Expression[*expression.Environment, bool]
	if options.GetFilterQuery() != "" {
		parser, err := expression.NewBotInstanceExpressionParser()
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		exp, err = parser.Parse(options.GetFilterQuery())
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	filterFn := options.GetFilterFn()
	if options.GetFilterSearchTerm() == "" && exp == nil && filterFn == nil {
		r, nextToken, err := service.ListResources(ctx, pageSize, lastKey)
		return r, nextToken, trace.Wrap(err)
	}

	r, nextToken, err := service.ListResourcesWithFilter(ctx, pageSize, lastKey, func(item *machineidv1.BotInstance) bool {
		if !services.MatchBotInstance(item, "", options.GetFilterSearchTerm(), exp) {
			return false
		}
		if filterFn != nil {
			return filterFn(item)
		}
		return true
	})

	return r, nextToken, trace.Wrap(err)
}

// DeleteBotInstance deletes a specific bot instance matching the given bot
// scope, bot name and instance ID. The scope must be empty if the owning bot
// is unscoped.
func (b *BotInstanceService) DeleteBotInstance(ctx context.Context, botScope, botName, instanceID string) error {
	serviceWithPrefix, err := b.serviceForBot(botScope, botName)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(serviceWithPrefix.DeleteResource(ctx, instanceID))
}

// DeleteAllBotInstances deletes all bot instances for all bots
func (b *BotInstanceService) DeleteAllBotInstances(ctx context.Context) error {
	return trace.Wrap(b.service.DeleteAllResources(ctx))
}

// PatchBotInstance uses the supplied function to patch the bot instance
// matching the given (botScope, botName, instanceID) key and persists the
// patched resource. It will make multiple attempts if a `CompareFailed` error
// is raised, automatically re-applying `updateFn()`.
func (b *BotInstanceService) PatchBotInstance(
	ctx context.Context,
	botScope, botName, instanceID string,
	updateFn func(*machineidv1.BotInstance) (*machineidv1.BotInstance, error),
) (*machineidv1.BotInstance, error) {
	const iterLimit = 3

	for range iterLimit {
		existing, err := b.GetBotInstance(ctx, botScope, botName, instanceID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		updated, err := updateFn(utils.CloneProtoMsg(existing))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		switch {
		case updated.GetMetadata().GetName() != existing.GetMetadata().GetName():
			return nil, trace.BadParameter("metadata.name: cannot be patched")
		case updated.GetMetadata().GetRevision() != existing.GetMetadata().GetRevision():
			return nil, trace.BadParameter("metadata.revision: cannot be patched")
		case updated.GetSpec().GetInstanceId() != existing.GetSpec().GetInstanceId():
			return nil, trace.BadParameter("spec.instance_id: cannot be patched")
		case updated.GetSpec().GetBotName() != existing.GetSpec().GetBotName():
			return nil, trace.BadParameter("spec.bot_name: cannot be patched")
		case updated.GetScope() != existing.GetScope():
			return nil, trace.BadParameter("scope: cannot be patched")
		}

		serviceWithPrefix, err := b.serviceForBot(botScope, botName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		lease, err := serviceWithPrefix.ConditionalUpdateResource(ctx, updated)
		if err != nil {
			if trace.IsCompareFailed(err) {
				continue
			}

			return nil, trace.Wrap(err)
		}

		updated.GetMetadata().SetRevision(lease.GetMetadata().GetRevision())
		return updated, nil
	}

	return nil, trace.CompareFailed("failed to update bot instance within %v iterations", iterLimit)
}
