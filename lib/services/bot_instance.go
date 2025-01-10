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

package services

import (
	"context"

	"github.com/gravitational/trace"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
)

// BotInstance is an interface for the BotInstance service.
type BotInstance interface {
	// CreateBotInstance
	CreateBotInstance(ctx context.Context, botInstance *machineidv1.BotInstance) (*machineidv1.BotInstance, error)

	// GetBotInstance
	GetBotInstance(ctx context.Context, botName, instanceID string) (*machineidv1.BotInstance, error)

	// ListBotInstances
	ListBotInstances(ctx context.Context, botName string, pageSize int, lastToken string) ([]*machineidv1.BotInstance, string, error)

	// DeleteBotInstance
	DeleteBotInstance(ctx context.Context, botName, instanceID string) error

	// PatchBotInstance fetches an existing bot instance by bot name and ID,
	// then calls `updateFn` to apply any changes before persisting the
	// resource.
	PatchBotInstance(
		ctx context.Context,
		botName, instanceID string,
		updateFn func(*machineidv1.BotInstance) (*machineidv1.BotInstance, error),
	) (*machineidv1.BotInstance, error)
}

// ValidateBotInstance verifies that required fields for a new BotInstance are present
func ValidateBotInstance(b *machineidv1.BotInstance) error {
	if b.Spec == nil {
		return trace.BadParameter("spec is required")
	}

	if b.Spec.BotName == "" {
		return trace.BadParameter("spec.bot_name is required")
	}

	if b.Spec.InstanceId == "" {
		return trace.BadParameter("spec.instance_id is required")
	}

	if b.Status == nil {
		return trace.BadParameter("status is required")
	}

	return nil
}

// MarshalBotInstance marshals the BotInstance object into a JSON byte array.
func MarshalBotInstance(object *machineidv1.BotInstance, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalBotInstance unmarshals the BotInstance object from a JSON byte array.
func UnmarshalBotInstance(data []byte, opts ...MarshalOption) (*machineidv1.BotInstance, error) {
	return UnmarshalProtoResource[*machineidv1.BotInstance](data, opts...)
}
