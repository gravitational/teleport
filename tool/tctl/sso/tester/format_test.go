/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package tester

import (
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
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
			got := FormatString(tt.description, tt.msg)
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
			object: types.RoleSpecV6{
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
  create_db_user: null
  create_desktop_user: null
  desktop_clipboard: null
  desktop_directory_sharing: null
  forward_agent: false
  pin_source_ip: false
  record_session: null
  ssh_file_copy: null
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatYAML(tt.description, tt.object)
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
			object: types.RoleSpecV6{
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
        "desktop_clipboard": null,
        "desktop_directory_sharing": null,
        "pin_source_ip": false,
        "ssh_file_copy": null,
        "create_desktop_user": null,
        "create_db_user": null
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
			got := FormatJSON(tt.description, tt.object)
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
   username: bar`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUserDetails(tt.description, tt.info)
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
