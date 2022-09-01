package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSortByFromString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in  string
		out SortBy
	}{
		{
			"hostname:asc",
			SortBy{
				Field:  "hostname",
				IsDesc: false,
			},
		},
		{
			"",
			SortBy{},
		},
		{
			"name:desc",
			SortBy{
				Field:  "name",
				IsDesc: true,
			},
		},
		{
			"hostname",
			SortBy{
				Field:  "hostname",
				IsDesc: false,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			out := GetSortByFromString(tt.in)
			require.Equal(t, tt.out, out)
		})
	}
}
