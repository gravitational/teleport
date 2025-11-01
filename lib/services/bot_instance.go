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
	"slices"

	"github.com/charlievieth/strcase"
	"github.com/gravitational/trace"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1/expression"
	"github.com/gravitational/teleport/lib/utils/typical"
)

// BotInstance is an interface for the BotInstance service.
type BotInstance interface {
	// CreateBotInstance
	CreateBotInstance(ctx context.Context, botInstance *machineidv1.BotInstance) (*machineidv1.BotInstance, error)

	// GetBotInstance
	GetBotInstance(ctx context.Context, botName, instanceID string) (*machineidv1.BotInstance, error)

	// ListBotInstances
	ListBotInstances(ctx context.Context, pageSize int, lastToken string, options *ListBotInstancesRequestOptions) ([]*machineidv1.BotInstance, string, error)

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

func MatchBotInstance(b *machineidv1.BotInstance, botName string, search string, exp typical.Expression[*expression.Environment, bool]) bool {
	if botName != "" && b.GetSpec().GetBotName() != botName {
		return false
	}

	heartbeat := GetBotInstanceLatestHeartbeat(b)
	authentication := GetBotInstanceLatestAuthentication(b)

	if exp != nil {
		if match, err := exp.Evaluate(&expression.Environment{
			Metadata:             b.GetMetadata(),
			Spec:                 b.GetSpec(),
			LatestHeartbeat:      heartbeat,
			LatestAuthentication: authentication,
		}); err != nil || !match {
			return false
		}
	}

	if search == "" {
		return true
	}

	values := []string{
		b.Spec.BotName,
		b.Spec.InstanceId,
	}

	if heartbeat != nil {
		values = append(values, heartbeat.Hostname, heartbeat.JoinMethod, heartbeat.Version, "v"+heartbeat.Version)
	}

	return slices.ContainsFunc(values, func(val string) bool {
		return strcase.Contains(val, search)
	})
}

// GetBotInstanceLatestHeartbeat returns the most recent heartbeat for the
// given bot instance.
func GetBotInstanceLatestHeartbeat(botInstance *machineidv1.BotInstance) *machineidv1.BotInstanceStatusHeartbeat {
	heartbeat := botInstance.GetStatus().GetInitialHeartbeat()
	latestHeartbeats := botInstance.GetStatus().GetLatestHeartbeats()
	if len(latestHeartbeats) > 0 {
		heartbeat = latestHeartbeats[len(latestHeartbeats)-1]
	}
	return heartbeat
}

// GetBotInstanceLatestAuthentication returns the most recent authentication for
// the given bot instance.
func GetBotInstanceLatestAuthentication(botInstance *machineidv1.BotInstance) *machineidv1.BotInstanceStatusAuthentication {
	authentication := botInstance.GetStatus().GetInitialAuthentication()
	latestAuthentications := botInstance.GetStatus().GetLatestAuthentications()
	if len(latestAuthentications) > 0 {
		authentication = latestAuthentications[len(latestAuthentications)-1]
	}
	return authentication
}

type ListBotInstancesRequestOptions struct {
	// The sort field to use for the results. If empty, the default sort field
	// is used.
	SortField string
	// The sort order to use for the results. If empty, the default sort order
	// is used.
	SortDesc bool
	// The name of the Bot to list BotInstances for. If empty, all BotInstances
	// will be listed.
	FilterBotName string
	// A search term used to filter the results. If non-empty, it's used to
	// match against supported fields.
	FilterSearchTerm string
	// A Teleport predicate language query used to filter the results.
	FilterQuery string
}

func (o *ListBotInstancesRequestOptions) GetSortField() string {
	if o == nil {
		return ""
	}
	return o.SortField
}

func (o *ListBotInstancesRequestOptions) GetSortDesc() bool {
	if o == nil {
		return false
	}
	return o.SortDesc
}

func (o *ListBotInstancesRequestOptions) GetFilterBotName() string {
	if o == nil {
		return ""
	}
	return o.FilterBotName
}

func (o *ListBotInstancesRequestOptions) GetFilterSearchTerm() string {
	if o == nil {
		return ""
	}
	return o.FilterSearchTerm
}

func (o *ListBotInstancesRequestOptions) GetFilterQuery() string {
	if o == nil {
		return ""
	}
	return o.FilterQuery
}
