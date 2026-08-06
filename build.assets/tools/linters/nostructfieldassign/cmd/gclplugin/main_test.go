package gclplugin

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	nostructfieldassign "github.com/gravitational/teleport/build.assets/tools/linters/nostructfieldassign"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name         string
		settings     any
		want         *Plugin
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name:         "nil settings",
			settings:     nil,
			want:         &Plugin{},
			errAssertion: require.NoError,
		},
		{
			name:         "missing fields",
			settings:     map[string]any{"other": "value"},
			want:         &Plugin{},
			errAssertion: require.NoError,
		},
		{
			name:     "invalid settings type",
			settings: "bad",
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "expected settings to be a map")
			},
		},
		{
			name: "invalid fields format",
			settings: map[string]any{
				"fields": "bad",
			},
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "invalid fields format")
			},
		},
		{
			name: "valid fields",
			settings: map[string]any{
				"fields": []map[string]any{
					{
						"pkg":   "example.com/mypkg",
						"type":  "MyStruct",
						"field": "Forbidden",
						"msg":   "use setter, not assignment",
					},
					{
						"pkg":   "github.com/aws/aws-sdk-go-v2/aws",
						"type":  "Config",
						"field": "Region",
					},
				},
			},
			errAssertion: require.NoError,
			want: &Plugin{
				rules: []nostructfieldassign.Rule{
					{
						Package:      "example.com/mypkg",
						Type:         "MyStruct",
						Field:        "Forbidden",
						ErrorMessage: "use setter, not assignment",
					},
					{
						Package: "github.com/aws/aws-sdk-go-v2/aws",
						Type:    "Config",
						Field:   "Region",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newPlugin(tt.settings)
			tt.errAssertion(t, err)
			if tt.want == nil {
				require.Nil(t, got)
			} else {
				require.Empty(t, cmp.Diff(tt.want, got, cmp.AllowUnexported(Plugin{})))
			}
		})
	}
}
