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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// filterItem collects values that the filters operate on so that we can use the
// same matchers for multiple types without having to complicate things with
// generics everywhere.
type filterItem struct {
	hasID bool
	id    string

	hasName bool
	name    string
}

// matcherFn defines the signature of a predicate over a [filterItem]
type matcherFn func(*filterItem) bool

// newIDMatcher creates a new matcherFn for exact matching of IDs
func newIDMatcher(id string) matcherFn {
	return func(item *filterItem) bool {
		return item.hasID && item.id == id
	}
}

// newRegexNameMatcher creates new matcherFn for pattern-based matching of
// item names.
func newRegexNameMatcher(nameRegex string) (matcherFn, error) {
	re, err := utils.CompileExpression(nameRegex)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(item *filterItem) bool {
		return item.hasName && re.MatchString(item.name)
	}, nil
}

// Filters is a set of subFilters that recognize which Identity Center resources
// to manage.
type Filters struct {
	include []matcherFn
	exclude []matcherFn
}

// Validate asserts the supplied collection protobuf `AWSICResourceFilter` objects
// represent a valid filter.
func Validate(inputFilters []*types.AWSICResourceFilter) error {
	_, err := New(inputFilters)
	return trace.Wrap(err)
}

// New compiles a new [Filters] instance from the supplied collection of
// [types.AWSICResourceFilter] objects.
func New(inputFilters []*types.AWSICResourceFilter) (Filters, error) {
	// The protobuf definition of a filter is a bit odd, and takes some
	// explaining before the validation makes sense.
	//
	// The YAML definition of a filter stack looks like this:
	//
	// filters:
	// - nameRegex: "^admin-.*$"
	// - id: "123"
	// - excludeId: "567"
	//
	// From this, the protobuf parser creates a stack of AWSICResourceFilter
	// objects, one for each predicate. Each AWSICResourceFilter has two
	// predicate slots: `include` and `exclude`. A well-formed AWSICResourceFilter
	// has only one filled predicate slot, with inclusive predicates in `Include`
	// and exclusive predicates in `Exclude`.
	//
	// In rare cases (most likely caused by an unknown predicate in the filter's
	// config text) the protobuf parser will give us an empty
	// `AWSICResourceFilter`. This is considered an error because the unknown
	// predicate could represent a mistyped exclude predicate, or a future-valid
	// exclusion rule in a cluster downgraded from a future version of Teleport;
	// just ignoring the unknown filter silently grant more access than the
	// user intends.

	var out Filters

	for index, filter := range inputFilters {
		if filter.Include == nil && filter.Exclude == nil {
			return Filters{}, trace.BadParameter("malformed filter predicate at index %d", index)
		}

		switch include := filter.Include.(type) {
		case *types.AWSICResourceFilter_NameRegex:
			matcher, err := newRegexNameMatcher(include.NameRegex)
			if err != nil {
				return Filters{}, trace.Wrap(err)
			}
			out.include = append(out.include, matcher)

		case *types.AWSICResourceFilter_Id:
			out.include = append(out.include, newIDMatcher(include.Id))

		case nil:
			// this must be an exclude predicate, but we're collecting includes
			// carry on

		default:
			// If we get to here, someone has added a new inclusion predicate
			// type to the protobuf definition and we have not updated to match.
			return Filters{}, trace.BadParameter("unsupported include predicate type: %T", include)
		}

		switch exclude := filter.Exclude.(type) {
		case *types.AWSICResourceFilter_ExcludeNameRegex:
			matcher, err := newRegexNameMatcher(exclude.ExcludeNameRegex)
			if err != nil {
				return Filters{}, trace.Wrap(err)
			}
			out.exclude = append(out.exclude, matcher)

		case *types.AWSICResourceFilter_ExcludeId:
			out.exclude = append(out.exclude, newIDMatcher(exclude.ExcludeId))

		case nil:
			// this must be an include predicate, but we're collecting excludes.
			// cary on.

		default:
			// If we get to here, someone has added a new exclusion predicate
			// type to the protobuf definition and we have not updated to match.
			return Filters{}, trace.BadParameter("unsupported exclude predicate type: %T", exclude)
		}
	}

	return out, nil
}

// Params holds extractor functions for the given item  type
type Params[T any] struct {
	// Items is the items to filter.
	Items []T
	// GetName is a function that gets the name of an item.
	GetName func(T) string
	// GetID is a function that gets the ID of an item.
	GetID func(T) string
}

// Filter applies the supplied filter stack to a collection of items,
// returning the filtered set. Note that Exclude rules are applied first
// regardless of the order the filters were defined in the original protobuf
// filter objects.
func Filter[T any](filters Filters, params Params[T]) []T {
	if len(filters.include) == 0 && len(filters.exclude) == 0 {
		return params.Items
	}

	if len(params.Items) == 0 {
		return params.Items
	}

	filterValues := filterItem{
		hasID:   params.GetID != nil,
		hasName: params.GetName != nil,
	}

	out := make([]T, 0, len(params.Items))
	for _, item := range params.Items {
		if filterValues.hasID {
			filterValues.id = params.GetID(item)
		}
		if filterValues.hasName {
			filterValues.name = params.GetName(item)
		}

		if filters.includeItem(&filterValues) {
			out = append(out, item)
		}
	}
	return out
}

func (fs *Filters) includeItem(item *filterItem) bool {
	for _, exclude := range fs.exclude {
		if exclude(item) {
			return false
		}
	}

	if len(fs.include) == 0 {
		return true
	}

	for _, include := range fs.include {
		if include(item) {
			return true
		}
	}
	return false
}
