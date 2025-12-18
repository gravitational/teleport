// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package filter

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// ErrUnknownFilter is returned when plugin sync filter
// encounters an unknown filter type.
var ErrUnknownFilter = &trace.BadParameterError{Message: "unknown PluginSyncFilter type"}

// Filters is a collection of PluginSyncFilter.
type Filters []*types.PluginSyncFilter

// New creates a new Filters instance from the supplied [types.PluginSyncFilter]s.
func New(r []*types.PluginSyncFilter) (Filters, error) {
	out := Filters(r)
	if err := out.validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// validate validates the Filters.
func (f Filters) validate() error {
	for _, v := range f {
		if v.GetInclude() != nil {
			switch filter := v.GetInclude().(type) {
			case *types.PluginSyncFilter_Id:
				if filter.Id == "" {
					return trace.BadParameter("id cannot be empty")
				}
			case *types.PluginSyncFilter_NameRegex:
				if _, err := utils.CompileExpression(v.GetNameRegex()); err != nil {
					return trace.Wrap(err)
				}
			default:
				return trace.WrapWithMessage(ErrUnknownFilter, "include filter: %T", filter)
			}
		}

		if v.GetExclude() != nil {
			switch filter := v.GetExclude().(type) {
			case *types.PluginSyncFilter_ExcludeId:
				if filter.ExcludeId == "" {
					return trace.BadParameter("exclude_id cannot be empty")
				}
			case *types.PluginSyncFilter_ExcludeNameRegex:
				if _, err := utils.CompileExpression(v.GetExcludeNameRegex()); err != nil {
					return trace.Wrap(err)
				}
			default:
				return trace.WrapWithMessage(ErrUnknownFilter, "exclude filter: %T", filter)
			}
		}
	}
	return nil
}

// MatchParam defines filter matcher parameters.
type MatchParam struct {
	// ID of an item to match.
	ID string
	// Name of an item to match.
	Name string
}

// Matches checks if the given [item] matches with the [filters].
// Exclude filters gets priority over Include filters.
// Empty [filters] implies a default match.
func Matches(filters Filters, param MatchParam) bool {
	if len(filters) == 0 {
		// Empty filter is considered a wildcard match.
		return true
	}

	for _, filter := range filters {
		if filter.GetExclude() == nil {
			continue
		}
		switch v := filter.GetExclude().(type) {
		case *types.PluginSyncFilter_ExcludeId:
			if param.ID != "" && param.ID == v.ExcludeId {
				return false
			}
		case *types.PluginSyncFilter_ExcludeNameRegex:
			if param.Name != "" {
				compiledExclude, err := utils.CompileExpression(v.ExcludeNameRegex)
				if err == nil && compiledExclude.MatchString(param.Name) {
					return false
				}
			}
		default:
			slog.ErrorContext(context.Background(), "PluginSyncFilter skipping an unknown include filter type", "type", logutils.TypeAttr(v))
		}
	}

	hasInclude := false
	for _, filter := range filters {
		if filter.GetInclude() == nil {
			continue
		}
		hasInclude = true
		switch v := filter.GetInclude().(type) {
		case *types.PluginSyncFilter_Id:
			if param.ID != "" && param.ID == v.Id {
				return true
			}
		case *types.PluginSyncFilter_NameRegex:
			if param.Name != "" {
				compiledFilter, err := utils.CompileExpression(v.NameRegex)
				if err == nil && compiledFilter.MatchString(param.Name) {
					return true
				}
			}
		default:
			slog.ErrorContext(context.Background(), "PluginSyncFilter skipping an unknown exclude filter type", "type", logutils.TypeAttr(v))
		}
	}
	return !hasInclude
}

// Inputs is used to configure filters
// in the Web UI or tctl. Field name matches with the
// include and exclude fields of PluginSyncFilter proto.
type Inputs struct {
	ID               []string `json:"id"`
	NameRegex        []string `json:"nameRegex"`
	ExcludeID        []string `json:"excludeId"`
	ExcludeNameRegex []string `json:"excludeNameRegex"`
}

// NewFromInputs returns a new [Filters] from [Inputs].
func NewFromInputs(in Inputs) (Filters, error) {
	cap := len(in.ID) + len(in.NameRegex) + len(in.ExcludeID) + len(in.ExcludeNameRegex)
	protoFilters := make([]*types.PluginSyncFilter, 0, cap)

	for _, id := range in.ID {
		protoFilters = append(protoFilters, &types.PluginSyncFilter{
			Include: &types.PluginSyncFilter_Id{Id: id},
		})
	}
	for _, n := range in.NameRegex {
		protoFilters = append(protoFilters, &types.PluginSyncFilter{
			Include: &types.PluginSyncFilter_NameRegex{NameRegex: n},
		})
	}
	for _, id := range in.ExcludeID {
		protoFilters = append(protoFilters, &types.PluginSyncFilter{
			Exclude: &types.PluginSyncFilter_ExcludeId{ExcludeId: id},
		})
	}
	for _, n := range in.ExcludeNameRegex {
		protoFilters = append(protoFilters, &types.PluginSyncFilter{
			Exclude: &types.PluginSyncFilter_ExcludeNameRegex{ExcludeNameRegex: n},
		})
	}

	filters, err := New(protoFilters)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return filters, nil
}
