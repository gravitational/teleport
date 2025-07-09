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

package filters

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// New creates a new Filters instance from the supplied [types.AWSICResourceFilter]s.
func New(filters []*types.AWSICResourceFilter) (Filters, error) {
	out := Filters(filters)
	if err := out.validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// Filters is a collection of filters.
type Filters []*types.AWSICResourceFilter

// validate validates the filters.
func (f Filters) validate() error {
	for _, v := range f {
		switch v.Include.(type) {
		case *types.AWSICResourceFilter_NameRegex:
			if _, err := utils.CompileExpression(v.GetNameRegex()); err != nil {
				return trace.Wrap(err)
			}
		}
		switch v.Exclude.(type) {
		case *types.AWSICResourceFilter_ExcludeNameRegex:
			if _, err := utils.CompileExpression(v.GetExcludeNameRegex()); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// Params is a collection of filter parameters.
// It contains the items to filter, and functions to get the name and ID of an item.
type Params[T any] struct {
	// Items is the items to filter.
	Items []T
	// GetName is a function that gets the name of an item.
	GetName func(T) string
	// GetID is a function that gets the ID of an item.
	GetID func(T) string
}

// Filter filters items based on the filters and parameters.
func Filter[T any](filters Filters, params Params[T]) []T {
	if len(filters) == 0 {
		return params.Items
	}
	var out []T
	for _, item := range params.Items {
		if matchesFilters(item, filters, params) {
			out = append(out, item)
		}
	}
	return out
}

func matchesFilters[T any](item T, filters Filters, params Params[T]) bool {
	hasInclude := false
	for _, filter := range filters {
		if filter.Exclude == nil {
			continue
		}
		switch v := filter.Exclude.(type) {
		case *types.AWSICResourceFilter_ExcludeId:
			if params.GetID != nil && params.GetID(item) == v.ExcludeId {
				return false
			}
		case *types.AWSICResourceFilter_ExcludeNameRegex:
			if params.GetName != nil {
				compiledExclude, err := utils.CompileExpression(v.ExcludeNameRegex)
				if err == nil && compiledExclude.MatchString(params.GetName(item)) {
					return false
				}
			}
		default:
			slog.ErrorContext(context.Background(), "AWSSyncFilter skipping unsupported exclude filter type", "type", logutils.TypeAttr(v))
		}
	}

	for _, filter := range filters {
		if filter.Include == nil {
			continue
		}
		hasInclude = true
		switch v := filter.Include.(type) {
		case *types.AWSICResourceFilter_Id:
			if params.GetID != nil && params.GetID(item) == v.Id {
				return true
			}
		case *types.AWSICResourceFilter_NameRegex:
			if params.GetName != nil {
				compiledFilter, err := utils.CompileExpression(v.NameRegex)
				if err == nil && compiledFilter.MatchString(params.GetName(item)) {
					return true
				}
			}
		default:
			slog.ErrorContext(context.Background(), "AWSSyncFilter unsupported filter type encountered. Filter will be skipped.", "type", logutils.TypeAttr(v))
		}
	}
	return !hasInclude
}
