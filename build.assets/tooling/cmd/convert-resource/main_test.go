package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
      - resources: ['session', 'event']
        verbs: ['list', 'read']
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
        }, {
        resources = ["session", "event"]
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
  audit:
    recurrence:
      frequency: 6months
  description: "Use this Access List to grant access to production to your engineers enrolled in the
support rotation."
  owners:
    - description: "manager of NA support team"
      name: alice
  ownership_requires:
    roles:
      - manager
  grants:
    roles:
      - support-engineer
  membership_requires:
    roles:
      - engineer
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
    description = "Use this Access List to grant access to production to your engineers enrolled in the support rotation."
    owners = [{
      description       = "manager of NA support team"
      ineligible_status = "0"
      membership_kind   = "0"
      name              = "alice"
    }]
    audit = {
      recurrence = {
        frequency    = "6"
        day_of_month = "0"
      }
    }
    membership_requires = {
      roles = ["engineer"]
    }
    ownership_requires = {
      roles = ["manager"]
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
  roles:
  - editor
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
    roles = ["editor"]
    traits = [{
      name   = "logins"
      values = ["root"]
    }]
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
spec:
  allow:
    rules:
      - resources: ['session', 'event']
        verbs: ['list', 'read']
---
kind: role
version: v8
metadata:
  name: manager2
spec:
  allow:
    rules:
      - resources: ['user', 'role']
        verbs: ['list','read']
`,
			expected: `resource "teleport_role" "manager1" {
  version = "v8"

  metadata = {
    name = "manager1"
  }

  spec = {
    allow = {
      rules = [{
        resources = ["session", "event"]
        verbs     = ["list", "read"]
      }]
    }
  }
}

resource "teleport_role" "manager2" {
  version = "v8"

  metadata = {
    name = "manager2"
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
spec:
  allow:
    rules:
      - resources: ['session', 'event']
        verbs: ['list', 'read']
---
kind: role
version: v8
metadata:
  name: manager2
spec:
  allow:
    rules:
      - resources: ['user', 'role']
        verbs: ['list','read']
`,
			expected: `apiVersion: resources.teleport.dev/v1
kind: TeleportRoleV8
metadata:
  name: manager1
spec:
  allow:
    rules:
      - resources:
          - session
          - event
        verbs:
          - list
          - read
---
apiVersion: resources.teleport.dev/v1
kind: TeleportRoleV8
metadata:
  name: manager2
spec:
  allow:
    rules:
      - resources:
          - user
          - role
        verbs:
          - list
          - read
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
