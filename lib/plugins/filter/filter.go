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

// validate validates the filters.
func (f Filters) validate() error {
	for _, v := range f {
		switch v.GetInclude().(type) {
		case *types.PluginSyncFilter_NameRegex:
			if _, err := utils.CompileExpression(v.GetNameRegex()); err != nil {
				return trace.Wrap(err)
			}
		}
		switch v.GetExclude().(type) {
		case *types.PluginSyncFilter_ExcludeNameRegex:
			if _, err := utils.CompileExpression(v.GetExcludeNameRegex()); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// MatchParam is a defines filter parameters.
// It contains the functions to get the name and ID of an item.
type MatchParam[T any] struct {
	// GetName is a function that gets the name of an item.
	GetName func(T) string
	// GetID is a function that gets the ID of an item.
	GetID func(T) string
}

// Matches checks if the given [item] matches with the [filters].
// Exclude filters gets priority over Include filters.
// Empty [filters] implies a default match.
func Matches[T any](item T, filters Filters, param MatchParam[T]) bool {
	if len(filters) == 0 {
		return false
	}
	hasInclude := false
	for _, filter := range filters {
		if filter.GetExclude() == nil {
			continue
		}
		switch v := filter.GetExclude().(type) {
		case *types.PluginSyncFilter_ExcludeId:
			if param.GetID != nil && param.GetID(item) == v.ExcludeId {
				return false
			}
		case *types.PluginSyncFilter_ExcludeNameRegex:
			if param.GetName != nil {
				compiledExclude, err := utils.CompileExpression(v.ExcludeNameRegex)
				if err == nil && compiledExclude.MatchString(param.GetName(item)) {
					return false
				}
			}
		default:
			slog.ErrorContext(context.Background(), "PluginSyncFilter skipping unsupported exclude filter type", "type", logutils.TypeAttr(v))
		}
	}

	for _, filter := range filters {
		if filter.GetInclude() == nil {
			continue
		}
		hasInclude = true
		switch v := filter.GetInclude().(type) {
		case *types.PluginSyncFilter_Id:
			if param.GetID != nil && param.GetID(item) == v.Id {
				return true
			}
		case *types.PluginSyncFilter_NameRegex:
			if param.GetName != nil {
				compiledFilter, err := utils.CompileExpression(v.NameRegex)
				if err == nil && compiledFilter.MatchString(param.GetName(item)) {
					return true
				}
			}
		default:
			slog.ErrorContext(context.Background(), "PluginSyncFilter unsupported filter type encountered. Filter will be skipped.", "type", logutils.TypeAttr(v))
		}
	}
	return !hasInclude
}
