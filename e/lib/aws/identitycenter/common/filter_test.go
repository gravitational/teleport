package identitycentercommon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestFilterItems(t *testing.T) {
	type TestItem struct {
		ID   string
		Name string
	}

	items := []TestItem{
		{ID: "1", Name: "apple"},
		{ID: "2", Name: "banana"},
		{ID: "3", Name: "cherry"},
		{ID: "4", Name: "avocado"},
	}

	tests := []struct {
		name     string
		filters  Filters
		params   FilterParams[TestItem]
		expected []TestItem
	}{
		{
			name:    "Filter by ID",
			filters: Filters{&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_Id{Id: "2"}}},
			params: FilterParams[TestItem]{
				Items: items,
				GetName: func(item TestItem) string {
					return item.Name
				},
				GetID: func(item TestItem) string {
					return item.ID
				},
			},
			expected: []TestItem{{ID: "2", Name: "banana"}},
		},
		{
			name:    "Filter by Name",
			filters: Filters{&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "a*"}}},
			params: FilterParams[TestItem]{
				Items: items,
				GetName: func(item TestItem) string {
					return item.Name
				},
				GetID: nil,
			},
			expected: []TestItem{{ID: "1", Name: "apple"}, {ID: "4", Name: "avocado"}},
		},
		{
			name:    "Exclude All",
			filters: Filters{&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "teleport.internal/exclude_all"}}},
			params: FilterParams[TestItem]{
				Items: items,
				GetName: func(item TestItem) string {
					return item.Name
				},
				GetID: func(item TestItem) string {
					return item.ID
				},
			},
			expected: nil,
		},
		{
			name:    "No Filters",
			filters: Filters{},
			params: FilterParams[TestItem]{
				Items: items,
				GetName: func(item TestItem) string {
					return item.Name
				},
				GetID: func(item TestItem) string {
					return item.ID
				},
			},
			expected: items,
		},
		{
			name: "Multiple Filters",
			filters: Filters{
				&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_Id{Id: "2"}},
				&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "a*"}},
				&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_Id{Id: "4"}},
			},
			params: FilterParams[TestItem]{
				Items: items,
				GetName: func(item TestItem) string {
					return item.Name
				},
				GetID: func(item TestItem) string {
					return item.ID
				},
			},
			expected: []TestItem{
				{ID: "1", Name: "apple"},
				{ID: "2", Name: "banana"},
				{ID: "4", Name: "avocado"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Filter(tt.filters, tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPoorlyFormedFiltersAreAnError(t *testing.T) {
	testCases := []struct {
		name           string
		filters        Filters
		errorAssertion require.ErrorAssertionFunc
	}{
		{
			name: "Bad regex",
			filters: Filters{
				&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "^[)$"}},
			},
			errorAssertion: require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			test.errorAssertion(t, test.filters.validate())
		})
	}
}
