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
			description: "access list",
			input: `version: v1
kind: access_list
metadata:
  name: support-engineers
spec:
  title: "Production access for support engineers"
  owners:
    - name: alice
  grants:
    roles:
      - support-engineer
`,
			expected: `resource "teleport_access_list" "support-engineers" {
  header = {
    kind    = "access_list"
    version = "v1"
    metadata = {
      name = "support-engineers"
    }
  }

  spec = {
    owners = [{
      ineligible_status = "0"
      membership_kind   = "0"
      name              = "alice"
    }]
    audit = {
      recurrence = {
        frequency    = "0"
        day_of_month = "0"
      }
    }
    grants = {
      roles = ["support-engineer"]
    }
    title = "Production access for support engineers"
  }
}
`,
		},
		{
			description: "rfd 153 resource",
			input: `kind: bot
version: v1
metadata:
  name: example
spec:
  traits:
  - name: logins
    values:
    - root
`,
			expected: `resource "teleport_bot" "example" {
  version = "v1"

  metadata = {
    name = "example"
  }

  spec = {
    traits = {
      logins = ["root"]
    }
  }
}
`,
		},
		{
			description: "false boolean",
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
			description: "openssh server",
			input: `kind: node
version: v2
sub_kind: openssh
metadata:
  name: a100fdd0-52db-4eca-a7ab-c3afa7a1564a
  labels:
    env: test
    team: engineering
spec:
  addr: <Var name="198.51.100.1:22" />
  hostname: <Var name="ssh-server-hostname" />
`,
			expected: `apiVersion: resources.teleport.dev/v1
kind: TeleportOpenSSHServerV2
metadata:
  labels:
    env: test
    team: engineering
  name: a100fdd0-52db-4eca-a7ab-c3afa7a1564a
spec:
  addr: <Var name="198.51.100.1:22" />
  hostname: <Var name="ssh-server-hostname" />
`,
		},
		{
			description: "github",
			input: `kind: github
metadata:
  name: github
spec:
  client_id: <client-id>
  client_secret: <client-secret>
  display: GitHub
  endpoint_url: ""
  redirect_url: https://<proxy-address>/v1/webapi/github/callback
  teams_to_logins: null
  teams_to_roles:
    - organization: ORG-NAME
      roles:
        - access
        - editor
      team: github-team
version: v3
`,
			expected: `apiVersion: resources.teleport.dev/v3
kind: TeleportGithubConnector
metadata:
  name: github
spec:
  client_id: <client-id>
  client_secret: <client-secret>
  display: GitHub
  endpoint_url: ""
  redirect_url: https://<proxy-address>/v1/webapi/github/callback
  teams_to_roles:
    - organization: ORG-NAME
      roles:
        - access
        - editor
      team: github-team
`,
		},
		{
			description: "description field",
			input: `kind: db
version: v3
metadata:
  name: example
  description: "Example database"
  labels:
    env: prod
    engine: postgres
spec:
  protocol: "postgres"
  uri: "localhost:5432"
`,
			expected: `apiVersion: resources.teleport.dev/v1
kind: TeleportDatabaseV3
metadata:
  annotations:
    description: Example database
  labels:
    engine: postgres
    env: prod
  name: example
spec:
  protocol: postgres
  uri: localhost:5432
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
