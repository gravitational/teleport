package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func Test_getBlockedPlugins(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want []types.PluginType
	}{
		{
			name: "no env",
			env:  "",
			want: nil,
		},
		{
			name: "single plugin",
			env:  types.PluginTypeAWSIdentityCenter,
			want: []types.PluginType{types.PluginTypeAWSIdentityCenter},
		},
		{
			name: "multiple plugins",
			env:  strings.Join([]string{types.PluginTypeAWSIdentityCenter, types.PluginTypeEntraID}, ","),
			want: []types.PluginType{types.PluginTypeAWSIdentityCenter, types.PluginTypeEntraID},
		},
		{
			name: "multiple plugins with whitespaces",
			env:  strings.Join([]string{types.PluginTypeAWSIdentityCenter, types.PluginTypeEntraID}, "  ,  "),
			want: []types.PluginType{types.PluginTypeAWSIdentityCenter, types.PluginTypeEntraID},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envVarNameDisabledPlugins, tt.env)
			require.ElementsMatch(t, tt.want, getDisabledPlugins())
		})
	}
}
