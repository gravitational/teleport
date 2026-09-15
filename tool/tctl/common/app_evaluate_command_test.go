/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/appresource"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/services"
)

const developerRole = `
kind: role
version: v9
metadata:
  name: developer
spec:
  allow:
    app_resources_expressions:
      - |
        path.match(literal("api/v4/projects", capture("project", greedy()))) &&
          deny_hint("project_not_allowed", "Project is not in the caller's allowlist",
            contains(user.traits["allowed_projects"], vars.project))
`

const aliceUser = `
kind: user
version: v2
metadata:
  name: alice
spec:
  roles: [developer]
  traits:
    allowed_projects: ["42", "99"]
`

const legacyRole = `
kind: role
version: v8
metadata:
  name: legacy
`

const allowAllRole = `
kind: role
version: v9
metadata:
  name: opener
spec:
  allow:
    app_resources:
      - allow_all: true
`

const denyRole = `
kind: role
version: v9
metadata:
  name: blocker
spec:
  deny:
    app_resources:
      - allow_all: true
`

const badExpressionRole = `
kind: role
version: v9
metadata:
  name: bad
spec:
  allow:
    app_resources_expressions:
      - user.role == "dev"
`

// userWithRoles returns a user document for alice holding roles.
func userWithRoles(roles ...string) string {
	return fmt.Sprintf(`
kind: user
version: v2
metadata:
  name: alice
spec:
  roles: [%s]
  traits:
    allowed_projects: ["42", "99"]
`, strings.Join(roles, ", "))
}

// writeSpec writes docs as one YAML file with a trailing document separator
// and returns its path.
func writeSpec(t *testing.T, docs ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.yaml")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()
	for _, doc := range docs {
		_, err := f.WriteString("---" + doc)
		require.NoError(t, err)
	}
	_, err = f.WriteString("---\n")
	require.NoError(t, err)
	return path
}

// evaluateResourceCmd returns an "apps evaluate-resource" command writing to
// stdout and stderr.
func evaluateResourceCmd(stdout, stderr *bytes.Buffer) *AppsCommand {
	flags := appsEvaluateFlags{stdout: stdout, stderr: stderr}
	return &AppsCommand{appsEvaluateResource: appsEvaluateResourceCommand{appsEvaluateFlags: flags}}
}

// evaluateRequestCmd returns an "apps evaluate-request" command writing to
// stdout and stderr.
func evaluateRequestCmd(stdout, stderr *bytes.Buffer) *AppsCommand {
	flags := appsEvaluateFlags{stdout: stdout, stderr: stderr}
	return &AppsCommand{appsEvaluateRequest: appsEvaluateRequestCommand{appsEvaluateFlags: flags}}
}

// runAppsEvaluateResource runs "tctl apps evaluate-resource" with args and
// returns the decoded decision, stderr, and the error.
func runAppsEvaluateResource(t *testing.T, args ...string) (map[string]any, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := runCommand(t, nil, evaluateResourceCmd(&stdout, &stderr), append([]string{"apps", "evaluate-resource"}, args...))
	if err != nil {
		return nil, stderr.String(), err
	}
	return decodeDecision(t, &stdout), stderr.String(), nil
}

// runAppsEvaluateRequest runs "tctl apps evaluate-request" with args and
// returns the decoded decision, stderr, and the error.
func runAppsEvaluateRequest(t *testing.T, clt *authclient.Client, args ...string) (map[string]any, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := runCommand(t, clt, evaluateRequestCmd(&stdout, &stderr), append([]string{"apps", "evaluate-request"}, args...))
	if err != nil {
		return nil, stderr.String(), err
	}
	return decodeDecision(t, &stdout), stderr.String(), nil
}

// decodeDecision returns the YAML decision written to stdout.
func decodeDecision(t *testing.T, stdout *bytes.Buffer) map[string]any {
	t.Helper()
	var decision map[string]any
	require.NoError(t, yaml.Unmarshal(stdout.Bytes(), &decision))
	return decision
}

// allowedWithVars and deniedWithHint are the two decisions developerRole
// returns for a path under /api/v4/projects.
var allowedWithVars = map[string]any{
	"allowed":    true,
	"roles":      []any{"developer"},
	"allow_role": "developer",
	"vars":       map[string]any{"project": "42"},
}

var deniedWithHint = map[string]any{
	"allowed":   false,
	"roles":     []any{"developer"},
	"deny_kind": "teleport_request_not_allowed",
	"hints": []any{map[string]any{
		"code":   "project_not_allowed",
		"reason": "Project is not in the caller's allowlist",
	}},
}

func TestAppsEvaluateResource(t *testing.T) {
	spec := writeSpec(t, developerRole, aliceUser)
	tests := []struct {
		name string
		args []string
		want map[string]any
	}{
		{
			name: "allowed with vars",
			args: []string{"/api/v4/projects/42/issues"},
			want: allowedWithVars,
		},
		{
			name: "denied with hint",
			args: []string{"/api/v4/projects/7/issues"},
			want: deniedWithHint,
		},
		{
			name: "space in the path is escaped",
			args: []string{"/api/v4/projects/4 2/issues"},
			want: deniedWithHint,
		},
		{
			name: "invalid path",
			args: []string{"/api/../secret"},
			want: map[string]any{
				"allowed":   false,
				"roles":     []any{"developer"},
				"deny_kind": "teleport_invalid_request",
			},
		},
		{
			name: "unsupported method",
			args: []string{"/api/v4/projects/42/issues", "--method=BREW"},
			want: map[string]any{
				"allowed":   false,
				"roles":     []any{"developer"},
				"deny_kind": "teleport_invalid_request",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, stderr, err := runAppsEvaluateResource(t, append([]string{"--spec=" + spec}, tt.args...)...)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Empty(t, stderr)
		})
	}
}

func TestAppsEvaluateResourceJSON(t *testing.T) {
	spec := writeSpec(t, developerRole, aliceUser)
	var stdout, stderr bytes.Buffer
	cmd := evaluateResourceCmd(&stdout, &stderr)
	err := runCommand(t, nil, cmd, []string{"apps", "evaluate-resource", "--spec=" + spec, "/api/v4/projects/42/issues", "--format=json"})
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
	require.Equal(t, allowedWithVars, got)
	require.Empty(t, stderr.String())
}

func TestAppsEvaluateResourceRoleVersions(t *testing.T) {
	pathsRole := `
kind: role
version: v9
metadata:
  name: paths
spec:
  allow:
    app_resources:
      - paths: ["/api/v4/projects/{project}/**"]
        where: contains(user.traits["allowed_projects"], vars.project)
`
	rolesRole := `
kind: role
version: v9
metadata:
  name: holder
spec:
  allow:
    app_resources:
      - paths: ["/**"]
        where: contains(user.roles, "holder")
`
	tests := []struct {
		name       string
		docs       []string
		want       map[string]any
		wantStderr string
	}{
		{
			name: "without a user every role is in user.roles",
			docs: []string{rolesRole},
			want: map[string]any{
				"allowed":    true,
				"roles":      []any{"holder"},
				"allow_role": "holder",
			},
		},
		{
			name: "spec role the user does not hold is dropped",
			docs: []string{allowAllRole, developerRole, userWithRoles("developer")},
			want: map[string]any{
				"allowed":    true,
				"roles":      []any{"developer"},
				"allow_role": "developer",
				"vars":       map[string]any{"project": "42"},
			},
		},
		{
			name:       "user with no role",
			docs:       []string{developerRole, userWithRoles()},
			want:       map[string]any{"allowed": false, "deny_kind": "teleport_request_not_allowed"},
			wantStderr: "Warning: no role to evaluate\n",
		},
		{
			name: "path rule",
			docs: []string{pathsRole, userWithRoles("paths")},
			want: map[string]any{
				"allowed":    true,
				"roles":      []any{"paths"},
				"allow_role": "paths",
				"vars":       map[string]any{"project": "42"},
			},
		},
		{
			name: "comment-only document is skipped",
			docs: []string{allowAllRole, "\n# just a comment\n"},
			want: map[string]any{
				"allowed":    true,
				"roles":      []any{"opener"},
				"allow_role": "opener",
			},
		},
		{
			name: "no user document",
			docs: []string{developerRole},
			want: deniedWithHint,
		},
		{
			name: "pre-v9 roles only",
			docs: []string{legacyRole},
			want: map[string]any{
				"allowed": true,
				"roles":   []any{"legacy"},
			},
			wantStderr: "Warning: every role predates v9, so app_resources rules do not apply\n",
		},
		{
			name: "pre-v9 role dropped beside a v9 role",
			docs: []string{legacyRole, developerRole, userWithRoles("legacy", "developer")},
			want: map[string]any{
				"allowed":    true,
				"roles":      []any{"developer"},
				"allow_role": "developer",
				"vars":       map[string]any{"project": "42"},
			},
			wantStderr: "Warning: dropped pre-v9 roles \"legacy\", because a v9 role is present\n",
		},
		{
			name: "deny-side rule denies beside allow_all",
			docs: []string{allowAllRole, denyRole},
			want: map[string]any{
				"allowed":   false,
				"roles":     []any{"blocker", "opener"},
				"deny_kind": "teleport_role_version_unsupported",
			},
			wantStderr: "Warning: role \"blocker\" sets app_resources or app_resources_expressions under deny, so the request is denied\n",
		},
		{
			name: "deny-side rule beside an expression",
			docs: []string{developerRole, denyRole, userWithRoles("developer", "blocker")},
			want: map[string]any{
				"allowed":   false,
				"roles":     []any{"blocker", "developer"},
				"deny_kind": "teleport_role_version_unsupported",
			},
			wantStderr: "Warning: role \"blocker\" sets app_resources or app_resources_expressions under deny, so the request is denied\n",
		},
		{
			name: "expression does not compile",
			docs: []string{badExpressionRole},
			want: map[string]any{
				"allowed":   false,
				"roles":     []any{"bad"},
				"deny_kind": "teleport_role_version_unsupported",
			},
			wantStderr: "Warning: role \"bad\" app_resources_expressions 0, so the role is not evaluated\n",
		},
		{
			name: "expression does not compile beside allow_all",
			docs: []string{allowAllRole, badExpressionRole},
			want: map[string]any{
				"allowed":    true,
				"roles":      []any{"opener"},
				"allow_role": "opener",
			},
			wantStderr: "Warning: role \"bad\" app_resources_expressions 0, so the role is not evaluated\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := writeSpec(t, tt.docs...)
			got, stderr, err := runAppsEvaluateResource(t, "--spec="+spec, "/api/v4/projects/42/issues")
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.wantStderr, stderr)
		})
	}
}

func TestAppsEvaluateResourceErrors(t *testing.T) {
	nodeDoc := `
kind: node
version: v2
metadata:
  name: host
`
	noKindDoc := `
version: v9
metadata:
  name: nokind
`
	tests := []struct {
		name    string
		docs    []string
		args    []string
		wantErr string
	}{
		{
			name:    "path with a host",
			docs:    []string{developerRole},
			args:    []string{"//evil.example.com/x"},
			wantErr: `PATH "//evil.example.com/x" must be a path only`,
		},
		{
			name:    "path with a query",
			docs:    []string{developerRole},
			args:    []string{"/x?y=1"},
			wantErr: `PATH "/x?y=1" must be a path only`,
		},
		{
			name:    "no role",
			docs:    []string{aliceUser},
			args:    []string{"/x"},
			wantErr: "the spec contains no role",
		},
		{
			name:    "two users",
			docs:    []string{developerRole, aliceUser, aliceUser},
			args:    []string{"/x"},
			wantErr: "the spec contains more than one user",
		},
		{
			name:    "other kind",
			docs:    []string{developerRole, nodeDoc},
			args:    []string{"/x"},
			wantErr: `the spec document 2 has kind "node"`,
		},
		{
			name:    "document without a kind",
			docs:    []string{developerRole, noKindDoc},
			args:    []string{"/x"},
			wantErr: "the spec document 2 has no kind",
		},
		{
			name:    "role the user holds is not in the spec",
			docs:    []string{developerRole, userWithRoles("developer", "missing")},
			args:    []string{"/x"},
			wantErr: `the spec contains no role "missing", which the user holds`,
		},
		{
			name:    "two roles with one name",
			docs:    []string{developerRole, developerRole},
			args:    []string{"/x"},
			wantErr: `the spec contains more than one role named "developer"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"--spec=" + writeSpec(t, tt.docs...)}, tt.args...)
			_, _, err := runAppsEvaluateResource(t, args...)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestDenyRulesDecision covers a role with rules under deny beside a role
// with allow_all, which does not allow the request. validateAppResources
// rejects a role with rules under deny on write, so the cluster cannot store
// one.
func TestDenyRulesDecision(t *testing.T) {
	opener := &types.RoleV6{Version: types.V9, Metadata: types.Metadata{Name: "opener"}}
	opener.Spec.Allow.AppResources = []types.AppResource{{AllowAll: true}}
	blocker := &types.RoleV6{Version: types.V9, Metadata: types.Metadata{Name: "blocker"}}
	blocker.Spec.Deny.AppResources = []types.AppResource{{AllowAll: true}}

	newerBlocker := &types.RoleV6{Version: "v10", Metadata: types.Metadata{Name: "newer-blocker"}}
	newerBlocker.Spec.Deny.AppResources = []types.AppResource{{AllowAll: true}}

	tests := []struct {
		name      string
		held      []types.Role
		wantNames []string
		want      string
	}{
		{
			name:      "v9 role",
			held:      []types.Role{opener, blocker},
			wantNames: []string{"blocker", "opener"},
			want:      "blocker",
		},
		{
			name:      "role above v9",
			held:      []types.Role{opener, newerBlocker},
			wantNames: []string{"newer-blocker", "opener"},
			want:      "newer-blocker",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			got, ok := denyRulesDecision(&stderr, tt.held)
			require.True(t, ok)
			want := appresource.Decision{
				Deny:  &appresource.DenyDetails{Kind: appresource.DenyRoleVersionUnsupported},
				Roles: tt.wantNames,
			}
			require.Equal(t, want, got)
			require.Equal(t, fmt.Sprintf("Warning: role %q sets app_resources or app_resources_expressions under deny, so the request is denied\n", tt.want), stderr.String())
		})
	}
	t.Run("no deny rules", func(t *testing.T) {
		var stderr bytes.Buffer
		_, ok := denyRulesDecision(&stderr, []types.Role{opener})
		require.False(t, ok)
		require.Empty(t, stderr.String())
	})
}

// TestEvaluateRolesVersionSkew covers roles that services.UnmarshalRole
// rejects, so they cannot come from a spec file.
func TestEvaluateRolesVersionSkew(t *testing.T) {
	newer := &types.RoleV6{Version: "v10", Metadata: types.Metadata{Name: "newer"}}
	opener := &types.RoleV6{Version: types.V9, Metadata: types.Metadata{Name: "opener"}}
	opener.Spec.Allow.AppResources = []types.AppResource{{AllowAll: true}}
	developer := &types.RoleV6{Version: types.V9, Metadata: types.Metadata{Name: "developer"}}
	developer.Spec.Allow.AppResourcesExpressions = []string{"true"}
	request := appresource.Request{Method: "GET", Path: "/x"}

	tests := []struct {
		name     string
		granting []types.Role
		want     appresource.Decision
	}{
		{
			name:     "newer role alone",
			granting: []types.Role{newer},
			want: appresource.Decision{
				Deny:  &appresource.DenyDetails{Kind: appresource.DenyRoleVersionUnsupported},
				Roles: []string{"newer"},
			},
		},
		{
			name:     "newer role beside allow_all",
			granting: []types.Role{newer, opener},
			want: appresource.Decision{
				Allowed: true,
				Allow:   &appresource.AllowDetails{Role: "opener"},
				Roles:   []string{"opener"},
			},
		},
		{
			name:     "newer role beside a matching expression",
			granting: []types.Role{newer, developer},
			want: appresource.Decision{
				Deny:  &appresource.DenyDetails{Kind: appresource.DenyRoleVersionUnsupported},
				Roles: []string{"developer", "newer"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			got, err := evaluateRoles(&stderr, tt.granting, request, appresource.Identity{Name: "alice"})
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Contains(t, stderr.String(), `role "newer" has unknown role version "v10"`)
		})
	}
}

// TestAppsEvaluateRequest evaluates against a cluster as the user alice, whose
// roles grant five HTTP apps and one GCP app, and grant and deny a sixth HTTP
// app. Two of the HTTP apps share one public address. The v9 role for the
// other apps sets no rule, so no stored role reads the request path.
func TestAppsEvaluateRequest(t *testing.T) {
	ctx := t.Context()
	authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })
	tlsServer, err := authServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tlsServer.Close()) })
	auth := tlsServer.Auth()

	for _, app := range []struct{ name, publicAddr, vendor, uri, cloud string }{
		{name: "gitlab", publicAddr: "gitlab.example.com", vendor: "gitlab", uri: "http://localhost:18080"},
		{name: "jira", vendor: "jira", uri: "http://localhost:18081"},
		{name: "jira.dev", vendor: "gitlab", uri: "http://localhost:18084"},
		{name: "gcp", publicAddr: "gcp.example.com", vendor: "gcp", uri: "https://console.cloud.google.com", cloud: types.CloudGCP},
		{name: "shop-web", publicAddr: "shop.example.com", vendor: "jira", uri: "http://localhost:18082"},
		{name: "shop-api", publicAddr: "shop.example.com", vendor: "gitlab", uri: "http://localhost:18083"},
		{name: "wiki", publicAddr: "wiki.example.com", vendor: "wiki", uri: "http://localhost:18085"},
	} {
		appV3, err := types.NewAppV3(types.Metadata{Name: app.name, Labels: map[string]string{"vendor": app.vendor}}, types.AppSpecV3{URI: app.uri, PublicAddr: app.publicAddr, Cloud: app.cloud})
		require.NoError(t, err)
		server, err := types.NewAppServerV3FromApp(appV3, "host", "host-id")
		require.NoError(t, err)
		_, err = auth.UpsertApplicationServer(ctx, server)
		require.NoError(t, err)
	}
	// Apps with no public_addr are reached as <name>.teleport.example.com.
	// The second proxy address is a suffix of the first. The longer suffix
	// wins, so jira.dev.teleport.example.com is jira.dev, not jira.dev.teleport.
	for name, addr := range map[string]string{"proxy": "teleport.example.com:443", "wide": "example.com:443"} {
		proxy, err := types.NewServer(name, types.KindProxy, types.ServerSpecV2{PublicAddrs: []string{addr}})
		require.NoError(t, err)
		_, err = auth.UpsertProxyServer(ctx, proxy)
		require.NoError(t, err)
	}

	// The opener role's app_labels are a trait template, so it grants the apps
	// labeled vendor: gitlab only once the user's traits are applied. The
	// wiki-denied role grants the wiki app and denies it again, so the auth
	// server leaves that app out of GetApplicationServers.
	for name, spec := range map[string]types.RoleSpecV6{
		"opener": {Allow: types.RoleConditions{
			AppLabels:    types.Labels{"vendor": []string{"{{external.vendor}}"}},
			AppResources: []types.AppResource{{AllowAll: true}},
			Rules:        []types.Rule{types.NewRule(types.KindAppServer, services.RO())},
		}},
		"no-rules": {Allow: types.RoleConditions{AppLabels: types.Labels{"vendor": []string{"jira", "gcp"}}}},
		"wiki-denied": {
			Allow: types.RoleConditions{AppLabels: types.Labels{"vendor": []string{"wiki"}}, AppResources: []types.AppResource{{AllowAll: true}}},
			Deny:  types.RoleConditions{AppLabels: types.Labels{"vendor": []string{"wiki"}}},
		},
	} {
		role, err := types.NewRoleWithVersion(name, types.V9, spec)
		require.NoError(t, err)
		_, err = auth.UpsertRole(ctx, role)
		require.NoError(t, err)
	}

	alice, err := types.NewUser("alice")
	require.NoError(t, err)
	alice.SetRoles([]string{"opener", "no-rules", "wiki-denied"})
	alice.SetTraits(map[string][]string{"vendor": {"gitlab"}})
	_, err = auth.UpsertUser(ctx, alice)
	require.NoError(t, err)
	clt, err := tlsServer.NewClient(authtest.TestUser("alice"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clt.Close()) })

	tests := []struct {
		name       string
		args       []string
		want       map[string]any
		wantStderr string
		wantErr    string
	}{
		{
			name: "allow_all role grants the app through a trait template",
			args: []string{"https://gitlab.example.com/api/v4/projects/42"},
			want: map[string]any{
				"allowed":    true,
				"roles":      []any{"opener"},
				"allow_role": "opener",
			},
		},
		{
			name: "app addressed by its name as the first label",
			args: []string{"https://jira.teleport.example.com/browse/X-1"},
			want: map[string]any{
				"allowed":   false,
				"roles":     []any{"no-rules"},
				"deny_kind": "teleport_request_not_allowed",
			},
		},
		{
			name: "app name with a dot wins over its first label",
			args: []string{"https://jira.dev.teleport.example.com/browse/X-1"},
			want: map[string]any{
				"allowed":    true,
				"roles":      []any{"opener"},
				"allow_role": "opener",
			},
		},
		{
			name:    "cloud app is not governed",
			args:    []string{"https://gcp.example.com/"},
			wantErr: `app_resources rules do not govern app "gcp"`,
		},
		{
			name:    "host outside the proxy address",
			args:    []string{"https://jira.unrelated.example/"},
			wantErr: `host "jira.unrelated.example" is not an app public address or a subdomain of a proxy public address`,
		},
		{
			name:    "unknown app",
			args:    []string{"https://nope.teleport.example.com/"},
			wantErr: `no app matches host "nope.teleport.example.com"`,
		},
		{
			name:    "deny app_labels in one role hides the app",
			args:    []string{"https://wiki.example.com/"},
			wantErr: `no app matches host "wiki.example.com"`,
		},
		{
			name: "two apps share the public address",
			args: []string{"https://shop.example.com/"},
			want: map[string]any{
				"allowed":    true,
				"roles":      []any{"opener"},
				"allow_role": "opener",
			},
			wantStderr: `Warning: apps "shop-api", "shop-web" share host "shop.example.com", so "shop-api" is evaluated`,
		},
		{
			name: "app name under the proxy selects one of the two",
			args: []string{"https://shop-api.teleport.example.com/"},
			want: map[string]any{
				"allowed":    true,
				"roles":      []any{"opener"},
				"allow_role": "opener",
			},
		},
		{
			name:    "url without a scheme",
			args:    []string{"gitlab.example.com/api"},
			wantErr: "URL must be absolute and have a host",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, stderr, err := runAppsEvaluateRequest(t, clt, tt.args...)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			if tt.wantStderr == "" {
				require.Empty(t, stderr)
				return
			}
			require.Contains(t, stderr, tt.wantStderr)
		})
	}
}

func TestParseSpec(t *testing.T) {
	tests := []struct{ name, in, wantErr string }{
		{name: "json role", in: `{"kind":"role","version":"v9","metadata":{"name":"r"},"spec":{}}`},
		{name: "not yaml", in: "kind: [", wantErr: "yaml"},
		{name: "user without a name", in: developerRole + "---\nkind: user\nversion: v2\nmetadata: {}\n", wantErr: "missing parameter Name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSpec(strings.NewReader(tt.in))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
