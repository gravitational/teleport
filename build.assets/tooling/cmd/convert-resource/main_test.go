// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_fieldPaths(t *testing.T) {
	type testCase struct {
		description string
		input       map[string]any
		expected    []string
	}

	cases := []testCase{
		{
			description: "two levels of scalars",
			input: map[string]any{
				"number":  0,
				"boolean": false,
				"string":  "",
				"object": map[string]any{
					"number":  0,
					"boolean": false,
					"string":  "",
				},
			},
			expected: []string{
				"number",
				"boolean",
				"string",
				"object",
				"object.number",
				"object.boolean",
				"object.string",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			actual := fieldPaths(c.input)
			assert.ElementsMatch(t, c.expected, actual)
		})
	}
}

func Test_convertYAMLToHCL(t *testing.T) {
	type testCase struct {
		description string
		input       string
		expected    string
	}

	cases := []testCase{
		{
			description: "simple role",
			input: `kind: role
version: v8
metadata:
  name: manager
spec:
  allow:
    rules:
      - resources: ['user', 'role']
        verbs: ['list','read']
`,
			expected: `resource "teleport_role" "manager" {
  version = "v8"

  metadata = {
    name = "manager"
  }

  spec = {
    allow = {
      rules = [{
        resources = ["user", "role"]
        verbs     = ["list", "read"]
      }]
    }
  }
}
`,
		},
		{
			description: "false boolean in a role",
			input: `kind: role
version: v8
metadata:
  name: myrole
spec:
  options:
    forward_agent: false`,
			expected: `resource "teleport_role" "myrole" {
  version = "v8"

  metadata = {
    name = "myrole"
  }

  spec = {
    options = {
      forward_agent = false
    }
  }
}
`,
		},
		{
			description: "token",
			input: `kind: token
version: v2
metadata:
  name: github-bot
spec:
  join_method: github
  roles:
  - Bot
  bot_name: github-bot
  github:
    allow:
    - repository: "your-github-username/my-repo"
`,
			expected: `resource "teleport_provision_token" "github-bot" {
  version = "v2"

  metadata = {
    name = "github-bot"
  }

  spec = {
    roles = ["Bot"]
    join_method = "github"
    bot_name = "github-bot"
    github = {
      allow = [{
        repository = "your-github-username/my-repo"
      }]
    }
  }
}
`,
		},
		{
			description: "cluster auth preference",
			input: `kind: cluster_auth_preference
version: v2
metadata:
  name: cluster-auth-preference
spec:
  type: local
  second_factors: ["otp", "webauthn"]
  require_session_mfa: hardware_key_touch
  disconnect_expired_cert: true
`,
			expected: `resource "teleport_auth_preference" "cluster-auth-preference" {
  version = "v2"

  metadata = {
    name = "cluster-auth-preference"
  }

  spec = {
    type = "local"
    disconnect_expired_cert = true
    require_session_mfa = 3
    second_factors = [1, 2]
  }
}
`,
		},
		{
			description: "user",
			input: `kind: user
version: v2
metadata:
  name: joe
spec:
  roles:
  - admin
  status:
    is_locked: false
    lock_expires: 0001-01-01T00:00:00Z
    locked_time: 0001-01-01T00:00:00Z
  traits:
    logins:
    - joe
    - root
  expires: 2025-01-01T00:00:00Z
  created_by:
    time: 2024-01-01T00:00:00Z
    user:
      name: builtin-Admin
`,
			expected: `resource "teleport_user" "joe" {
  version = "v2"

  metadata = {
    name = "joe"
  }

  spec = {
    roles = ["admin"]
    traits = {
      logins = ["joe", "root"]
    }
  }
}
`,
		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var buf bytes.Buffer
			err := convertYAMLToHCL(&buf, strings.NewReader(c.input))
			assert.NoError(t, err)
			assert.Equal(t, c.expected, buf.String())
		})
	}
}
func Test_convertAllYAMLToHCL(t *testing.T) {
	type testCase struct {
		description string
		input       string
		expected    string
	}

	cases := []testCase{
		{
			description: "two roles",
			input: `---
kind: role
version: v8
metadata:
  name: manager1
---
kind: role
version: v8
metadata:
  name: manager2
`,
			expected: `resource "teleport_role" "manager1" {
  version = "v8"

  metadata = {
    name = "manager1"
  }

}

resource "teleport_role" "manager2" {
  version = "v8"

  metadata = {
    name = "manager2"
  }

}
`,
		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var buf bytes.Buffer
			err := convertAllYAMLToHCL(&buf, strings.NewReader(c.input))
			assert.NoError(t, err)
			assert.Equal(t, c.expected, buf.String())
		})
	}
}

func Test_convertYAMLToKubernetes(t *testing.T) {
	type testCase struct {
		description string
		input       string
		expected    string
	}

	cases := []testCase{
		{
			description: "simple role",
			input: `kind: role
version: v8
metadata:
  name: manager
spec:
  allow:
    rules:
      - resources: ['user', 'role']
        verbs: ['list','read']
      - resources: ['session', 'event']
        verbs: ['list', 'read']
`,
			expected: `apiVersion: resources.teleport.dev/v1
kind: TeleportRoleV8
metadata:
  name: manager
spec:
  allow:
    rules:
      - resources:
          - user
          - role
        verbs:
          - list
          - read
      - resources:
          - session
          - event
        verbs:
          - list
          - read
`,
		},
		{
			description: "token",
			input: `kind: token
version: v2
metadata:
  name: github-bot
spec:
  join_method: github
  roles:
  - Bot
  bot_name: github-bot
  github:
    allow:
    - repository: "your-github-username/my-repo"
`,
			expected: `apiVersion: resources.teleport.dev/v2
kind: TeleportProvisionToken
metadata:
  name: github-bot
spec:
  bot_name: github-bot
  github:
    allow:
      - repository: your-github-username/my-repo
  join_method: github
  roles:
    - Bot
`,
		},
		{
			description: "user",
			input: `kind: user
version: v2
metadata:
  name: joe
spec:
  roles:
  - admin
  status:
    is_locked: false
    lock_expires: 0001-01-01T00:00:00Z
    locked_time: 0001-01-01T00:00:00Z
  traits:
    logins:
    - joe
    - root
  expires: 2025-01-01T00:00:00Z
  created_by:
    time: 2024-01-01T00:00:00Z
    user:
      name: builtin-Admin
`,
			expected: `apiVersion: resources.teleport.dev/v2
kind: TeleportUser
metadata:
  name: joe
spec:
  roles:
    - admin
  traits:
    logins:
      - joe
      - root
`,
		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var buf bytes.Buffer
			err := convertYAMLToKubernetes(&buf, strings.NewReader(c.input))
			assert.NoError(t, err)
			assert.Equal(t, c.expected, buf.String())
		})
	}
}

func Test_convertYAMLToKubernetes_errors(t *testing.T) {
	type testCase struct {
		description       string
		input             string
		expectedErrString string
	}

	cases := []testCase{
		{
			description: "outdated role version",
			input: `kind: role
version: v7
metadata:
  name: manager
`,
			expectedErrString: `Kubernetes does not support resource kind role (v7)`,
		},
		{
			description: "cluster auth preference unsupported for kubernetes",
			input: `kind: cluster_auth_preference
version: v2
metadata:
  name: cluster-auth-preference
`,
			expectedErrString: `Kubernetes does not support resource kind cluster_auth_preference`,
		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var buf bytes.Buffer
			err := convertYAMLToKubernetes(&buf, strings.NewReader(c.input))
			assert.ErrorContains(t, err, c.expectedErrString)
		})
	}
}
func Test_convertAllYAMLToKubernetes(t *testing.T) {
	type testCase struct {
		description string
		input       string
		expected    string
	}

	cases := []testCase{
		{
			description: "two roles",
			input: `---
kind: role
version: v8
metadata:
  name: manager1
---
kind: role
version: v8
metadata:
  name: manager2
`,
			expected: `apiVersion: resources.teleport.dev/v1
kind: TeleportRoleV8
metadata:
  name: manager1
---
apiVersion: resources.teleport.dev/v1
kind: TeleportRoleV8
metadata:
  name: manager2
`,
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var buf bytes.Buffer
			err := convertAllYAMLToKubernetes(&buf, strings.NewReader(c.input))
			assert.NoError(t, err)
			assert.Equal(t, c.expected, buf.String())
		})
	}
}
