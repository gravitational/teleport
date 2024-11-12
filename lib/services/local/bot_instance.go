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
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	botInstancePrefix = "bot_instance"
)

// BotInstanceService exposes backend functionality for storing bot instances.
type BotInstanceService struct {
	service *generic.ServiceWrapper[*machineidv1.BotInstance]

	clock clockwork.Clock
}

// NewBotInstanceService creates a new BotInstanceService with the given backend.
func NewBotInstanceService(b backend.Backend, clock clockwork.Clock) (*BotInstanceService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceWrapperConfig[*machineidv1.BotInstance]{
			Backend:       b,
			ResourceKind:  types.KindBotInstance,
			BackendPrefix: backend.NewKey(botInstancePrefix),
			MarshalFunc:   services.MarshalBotInstance,
			UnmarshalFunc: services.UnmarshalBotInstance,
			ValidateFunc:  services.ValidateBotInstance,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &BotInstanceService{
		service: service,
		clock:   clock,
	}, nil
}

// CreateBotInstance inserts a new BotInstance into the backend.
//
// Note that new BotInstances will have their .Metadata.Name overwritten by the
// instance UUID.
func (b *BotInstanceService) CreateBotInstance(ctx context.Context, instance *machineidv1.BotInstance) (*machineidv1.BotInstance, error) {
	instance.Kind = types.KindBotInstance
	instance.Version = types.V1

	if instance.Metadata == nil {
		instance.Metadata = &headerv1.Metadata{}
	}

	instance.Metadata.Name = instance.Spec.InstanceId

	serviceWithPrefix := b.service.WithPrefix(instance.Spec.BotName)
	created, err := serviceWithPrefix.CreateResource(ctx, instance)
	return created, trace.Wrap(err)
}

// GetBotInstance retreives a specific bot instance given a bot name and
// instance ID.
func (b *BotInstanceService) GetBotInstance(ctx context.Context, botName, instanceID string) (*machineidv1.BotInstance, error) {
	serviceWithPrefix := b.service.WithPrefix(botName)
	instance, err := serviceWithPrefix.GetResource(ctx, instanceID)
	return instance, trace.Wrap(err)
}

// ListBotInstances lists all bot instances matching the given bot name filter.
// If an empty bot name is provided, all bot instances will be fetched.
func (b *BotInstanceService) ListBotInstances(ctx context.Context, botName string, pageSize int, lastKey string) ([]*machineidv1.BotInstance, string, error) {
	// If botName is empty, return instances for all bots by not using a service prefix
	if botName == "" {
		r, nextToken, err := b.service.ListResources(ctx, pageSize, lastKey)
		return r, nextToken, trace.Wrap(err)
	}

	serviceWithPrefix := b.service.WithPrefix(botName)
	r, nextToken, err := serviceWithPrefix.ListResources(ctx, pageSize, lastKey)
	return r, nextToken, trace.Wrap(err)
}

// DeleteBotInstance deletes a specific bot instance matching the given bot name
// and instance ID.
func (b *BotInstanceService) DeleteBotInstance(ctx context.Context, botName, instanceID string) error {
	serviceWithPrefix := b.service.WithPrefix(botName)
	return trace.Wrap(serviceWithPrefix.DeleteResource(ctx, instanceID))
}

// PatchBotInstance uses the supplied function to patch the bot instance
// matching the given (botName, instanceID) key and persists the patched
// resource. It will make multiple attempts if a `CompareFailed` error is
// raised, automatically re-applying `updateFn()`.
func (b *BotInstanceService) PatchBotInstance(
	ctx context.Context,
	botName, instanceID string,
	updateFn func(*machineidv1.BotInstance) (*machineidv1.BotInstance, error),
) (*machineidv1.BotInstance, error) {
	const iterLimit = 3

	for i := 0; i < iterLimit; i++ {
		existing, err := b.GetBotInstance(ctx, botName, instanceID)
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
		}

		serviceWithPrefix := b.service.WithPrefix(botName)
		lease, err := serviceWithPrefix.ConditionalUpdateResource(ctx, updated)
		if err != nil {
			if trace.IsCompareFailed(err) {
				continue
			}

			return nil, trace.Wrap(err)
		}

		updated.GetMetadata().Revision = lease.GetMetadata().Revision
		return updated, nil
	}

	return nil, trace.CompareFailed("failed to update bot instance within %v iterations", iterLimit)
}
