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
	"strings"

	"github.com/charlievieth/strcase"
	"github.com/gravitational/trace"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1/expression"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/utils/typical"
)

// BotUserPrefix is the prefix appended to bot users. Users with this prefix are
// plausibly bots, but this prefix is not sufficient to establish that a given
// user actually is a bot user. The existence of [types.BotLabel] in the user
// labels is the definitive indicator that a user is a bot.
const BotUserPrefix = "bot-"

// BotInstance is an interface for the BotInstance service.
//
// Bot instances belong to a bot, and bots are identified by their scope and
// name: instances of scoped bots are stored in a scope-namespaced key range,
// separate from instances of unscoped bots. Methods addressing an individual
// instance take the owning bot's scope, which must be empty if the bot is
// unscoped.
type BotInstance interface {
	// CreateBotInstance creates a new bot instance. It is stored in the key
	// range determined by the scope set on the instance itself.
	CreateBotInstance(ctx context.Context, botInstance *machineidv1.BotInstance) (*machineidv1.BotInstance, error)

	// GetBotInstance returns the bot instance owned by the bot identified by
	// the request's (bot_scope, bot_name) with the given instance ID.
	GetBotInstance(ctx context.Context, req *machineidv1.GetBotInstanceRequest) (*machineidv1.BotInstance, error)

	// ListBotInstances
	ListBotInstances(ctx context.Context, pageSize int, lastToken string, options *ListBotInstancesRequestOptions) ([]*machineidv1.BotInstance, string, error)

	// DeleteBotInstance deletes the bot instance owned by the bot identified
	// by the request's (bot_scope, bot_name) with the given instance ID.
	DeleteBotInstance(ctx context.Context, req *machineidv1.DeleteBotInstanceRequest) error

	// PatchBotInstance fetches the existing bot instance identified by the
	// given options, then calls the options' UpdateFn to apply any changes
	// before persisting the resource.
	PatchBotInstance(ctx context.Context, opts PatchBotInstanceOpts) (*machineidv1.BotInstance, error)
}

// PatchBotInstanceOpts identifies the bot instance to be patched by
// [BotInstance.PatchBotInstance] and holds the patch to apply to it.
type PatchBotInstanceOpts struct {
	// Bot is the scope-qualified name of the bot that owns the instance. The
	// scope must be empty if the bot is unscoped.
	Bot scopes.QualifiedName
	// InstanceID is the ID of the instance to patch.
	InstanceID string
	// UpdateFn is applied to the fetched instance to produce the instance to
	// persist. It may be called more than once if the write is retried.
	UpdateFn func(*machineidv1.BotInstance) (*machineidv1.BotInstance, error)
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
	// The scope of the Bot to list BotInstances for. A bot is identified by the
	// pair (scope, name), so this only ever qualifies FilterBotName and must be
	// set alongside it; setting it without FilterBotName is an error. Leave
	// empty if the bot is unscoped. This is deliberately not a scope filter for
	// listing every BotInstance in a scope - use ScopeFilter for that.
	FilterBotScope string
	// ScopeFilter selects BotInstances by the scope of their owning bot. A nil or
	// MODE_UNSPECIFIED filter matches every scope; identity-derived defaulting is
	// the caller's job (see ScopedAccessCheckerContext.ResolveScopeFilter).
	//
	// Mutually exclusive with FilterBotName, which already constrains the result
	// to a single bot in a single scope.
	ScopeFilter *scopesv1.Filter
	// A search term used to filter the results. If non-empty, it's used to
	// match against supported fields.
	FilterSearchTerm string
	// A Teleport predicate language query used to filter the results.
	FilterQuery string
	// FilterFn is an optional additional filter applied during iteration.
	FilterFn func(*machineidv1.BotInstance) bool
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

func (o *ListBotInstancesRequestOptions) GetFilterBotScope() string {
	if o == nil {
		return ""
	}
	return o.FilterBotScope
}

func (o *ListBotInstancesRequestOptions) GetScopeFilter() *scopesv1.Filter {
	if o == nil {
		return nil
	}
	return o.ScopeFilter
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

func (o *ListBotInstancesRequestOptions) GetFilterFn() func(*machineidv1.BotInstance) bool {
	if o == nil {
		return nil
	}
	return o.FilterFn
}

// BotResourceName returns the default name for resources associated with the
// given bot. An empty Scope refers to an unscoped bot.
//
// Bots are namespaced by their scope, so a scoped bot's name encodes the scope
// as well as the bot name (bot-<encoded_scope>-<name>), allowing a name to be reused across
// scopes. An encoded scope only ever contains lowercase alphanumerics, so the "-" separator
// keeps the two apart and two different scopes cannot yield the same name. Scoped bots are
// reconstructed from User labels rather than by parsing this name, so it serves
// only as an identity key.
func BotResourceName(bot scopes.QualifiedName) (string, error) {
	name := bot.Name
	if bot.Scope != "" {
		encodedScope, err := scopes.EncodeForKey(bot.Scope)
		if err != nil {
			return "", trace.Wrap(err, "encoding scope for bot resource name")
		}
		name = encodedScope + "-" + bot.Name
	}
	return BotUserPrefix + strings.ReplaceAll(name, " ", "-"), nil
}
