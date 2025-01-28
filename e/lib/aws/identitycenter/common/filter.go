package identitycentercommon

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// NewFilters creates a new Filters instance from the supplied [types.AWSICResourceFilter]s.
func NewFilters(filters []*types.AWSICResourceFilter) (Filters, error) {
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
	}
	return nil
}

// FilterParams is a collection of filter parameters.
// It contains the items to filter, and functions to get the name and ID of an item.
type FilterParams[T any] struct {
	// Items is the items to filter.
	Items []T
	// GetName is a function that gets the name of an item.
	GetName func(T) string
	// GetID is a function that gets the ID of an item.
	GetID func(T) string
}

// Filter filters items based on the filters and parameters.
func Filter[T any](filters Filters, params FilterParams[T]) []T {
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

func matchesFilters[T any](item T, filters Filters, params FilterParams[T]) bool {
	for _, filter := range filters {
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
			slog.ErrorContext(context.Background(), "AWSSyncFilter unsupported filter type encountered. Filter will be skipped.", "type", fmt.Sprintf("%T", v))
		}
	}
	return false
}
