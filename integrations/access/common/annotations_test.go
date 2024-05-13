package common

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestGetServiceNamesFromAnnotations(t *testing.T) {
	testAnnotationKey := "test-key"
	tests := []struct {
		name        string
		annotations map[string][]string
		want        []string
		assertErr   require.ErrorAssertionFunc
	}{
		{
			name:        "Returns 'not found' when annotation is not present",
			annotations: map[string][]string{"other-key": {"foo", "bar"}},
			want:        nil,
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				expectedErr := &trace.NotFoundError{}
				require.ErrorAs(t, err, &expectedErr)
			},
		},
		{
			name:        "Returns 'bad parameter' when annotation is empty",
			annotations: map[string][]string{"test-key": nil},
			want:        nil,
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				expectedErr := &trace.BadParameterError{}
				require.ErrorAs(t, err, &expectedErr)
			},
		},
		{
			name:        "Single service name",
			annotations: map[string][]string{"test-key": {"foo"}},
			want:        []string{"foo"},
			assertErr:   require.NoError,
		},
		{
			name:        "Multiple service names",
			annotations: map[string][]string{"test-key": {"foo", "bar"}},
			want:        []string{"foo", "bar"},
			assertErr:   require.NoError,
		},
		{
			name:        "Duplicated service names are deduplicated",
			annotations: map[string][]string{"test-key": {"foo", "bar", "foo", "bar"}},
			want:        []string{"foo", "bar"},
			assertErr:   require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &types.AccessRequestV3{Spec: types.AccessRequestSpecV3{SystemAnnotations: tt.annotations}}
			got, err := GetServiceNamesFromAnnotations(request, testAnnotationKey)
			tt.assertErr(t, err)
			require.ElementsMatch(t, tt.want, got)
		})
	}
}
