package main

import (
	"fmt"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	"github.com/stretchr/testify/require"
)

func Test_formatString(t *testing.T) {
	tests := []struct {
		name        string
		description string
		msg         string
		want        string
	}{
		{
			name:        "empty",
			description: "",
			msg:         "",
			want:        ":\n\n",
		},
		{
			name:        "something",
			description: "a field",
			msg:         "foo baz bar blah",
			want: `a field:
foo baz bar blah
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatString(tt.description, tt.msg)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_formatYAML(t *testing.T) {
	tests := []struct {
		name        string
		description string
		object      interface{}
		want        string
	}{
		{
			name:        "empty",
			description: "",
			object:      nil,
			want:        ":\nnull\n",
		},
		{
			name:        "simple object",
			description: "my field",
			object: types.RoleSpecV5{
				Allow: types.RoleConditions{
					Logins:        []string{"username"},
					ClusterLabels: types.Labels{"access": []string{"ops"}},
				},
			},
			want: `my field:
allow:
  cluster_labels:
    access: ops
  logins:
  - username
deny: {}
options:
  cert_format: ""
  desktop_clipboard: null
  forward_agent: false
  record_session: null
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatYAML(tt.description, tt.object)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_formatJSON(t *testing.T) {
	tests := []struct {
		name        string
		description string
		object      interface{}
		want        string
	}{
		{
			name:        "empty",
			description: "empty field",
			object:      struct{}{},
			want: `empty field:
{}
`,
		},
		{
			name:        "simple object",
			description: "my field",
			object: types.RoleSpecV5{
				Allow: types.RoleConditions{
					Logins:        []string{"username"},
					ClusterLabels: types.Labels{"access": []string{"ops"}},
				},
			},
			want: `my field:
{
    "options": {
        "forward_agent": false,
        "cert_format": "",
        "record_session": null,
        "desktop_clipboard": null
    },
    "allow": {
        "logins": [
            "username"
        ],
        "cluster_labels": {
            "access": "ops"
        }
    },
    "deny": {}
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatJSON(tt.description, tt.object)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_formatUserDetails(t *testing.T) {
	tests := []struct {
		name        string
		description string
		info        *types.CreateUserParams
		want        string
	}{
		{
			name:        "empty",
			description: "",
			info:        nil,
			want:        "",
		},
		{
			name:        "some details",
			description: "user details",
			info: &types.CreateUserParams{
				ConnectorName: "foo",
				Username:      "bar",
				Logins:        []string{"laa", "lbb", "lcc"},
				KubeGroups:    []string{"kgaa", "kgbb", "kgcc"},
				KubeUsers:     []string{"kuaa", "kubb", "kucc"},
				Roles:         []string{"raa", "rbb", "rcc"},
				Traits: map[string][]string{
					"groups": {"gfoo", "gbar", "gbaz"},
				},
				SessionTTL: 1230,
			},
			want: `user details:
   kube_groups:
   - kgaa
   - kgbb
   - kgcc
   kube_users:
   - kuaa
   - kubb
   - kucc
   logins:
   - laa
   - lbb
   - lcc
   roles:
   - raa
   - rbb
   - rcc
   traits:
     groups:
     - gfoo
     - gbar
     - gbaz
   username: bar
   `,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUserDetails(tt.description, tt.info)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_formatSSOWarnings(t *testing.T) {
	tests := []struct {
		name        string
		description string
		info        *types.SSOWarnings
		want        string
	}{
		{
			name:        "empty",
			description: "",
			info:        nil,
			want:        "",
		},
		{
			name:        "message, no individual warnings",
			description: "my field",
			info:        &types.SSOWarnings{Message: "blah blah"},
			want:        "my field: blah blah\n",
		},
		{
			name:        "message and warnings",
			description: "my field",
			info: &types.SSOWarnings{
				Message:  "blah blah",
				Warnings: []string{"foo", "bar", "baz"},
			},
			want: "my field: blah blah. Warnings:\n- foo\n- bar\n- baz\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSSOWarnings(tt.description, tt.info)
			require.Equal(t, tt.want, got)
		})
	}

}

func Test_formatError(t *testing.T) {
	tests := []struct {
		name      string
		fieldDesc string
		err       error
		want      string
	}{
		{
			name:      "empty",
			fieldDesc: "my field",
			err:       nil,
			want:      "my field: error rendering field: <nil>\n",
		},
		{
			name:      "plain error",
			fieldDesc: "my field",
			err:       fmt.Errorf("foo: %v", 123),
			want:      "my field: error rendering field: foo: 123\n",
		},
		{
			name:      "trace error",
			fieldDesc: "my field",
			err:       trace.Errorf("bar: %v", 321),
			want:      "my field: error rendering field: bar: 321\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatError(tt.fieldDesc, tt.err)
			require.Equal(t, tt.want, got)
		})
	}
}
