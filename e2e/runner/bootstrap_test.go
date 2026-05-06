/**
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

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const viewerRoleYAML = `kind: role
version: v7
metadata:
  name: viewer
spec:
  allow:
    logins: ['root']
`

func TestReadRoleFile(t *testing.T) {
	e2eDir := t.TempDir()
	rolesDir := createDir(t, e2eDir, "testdata", "roles")
	writeFile(t, rolesDir, "viewer.yaml", viewerRoleYAML)

	cr, err := readRoleFile(e2eDir, "viewer.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cr.name != "viewer" {
		t.Errorf("name = %q, want %q", cr.name, "viewer")
	}

	if cr.YAML != viewerRoleYAML {
		t.Errorf("YAML content mismatch")
	}
}

func TestReadRoleFileMissingName(t *testing.T) {
	e2eDir := t.TempDir()
	rolesDir := createDir(t, e2eDir, "testdata", "roles")
	writeFile(t, rolesDir, "bad.yaml", `kind: role
version: v7
spec:
  allow:
    logins: ['root']
`)

	_, err := readRoleFile(e2eDir, "bad.yaml")
	if err == nil {
		t.Fatal("expected error for missing metadata.name, got nil")
	}
}

func TestReadRoleFileRejectsTraversal(t *testing.T) {
	e2eDir := t.TempDir()
	createDir(t, e2eDir, "testdata", "roles")

	// Sentinel outside the roles dir that a traversal would otherwise reach.
	writeFile(t, e2eDir, "secret.yaml", `kind: role
version: v7
metadata:
  name: secret
`)

	cases := []string{
		"../secret.yaml",
		"../../etc/passwd",
		"/etc/passwd",
	}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := readRoleFile(e2eDir, name); err == nil {
				t.Fatalf("expected error for filename %q, got nil", name)
			}
		})
	}
}

func TestBuildBootstrapState(t *testing.T) {
	e2eDir := t.TempDir()
	rolesDir := createDir(t, e2eDir, "testdata", "roles")
	writeFile(t, rolesDir, "viewer.yaml", viewerRoleYAML)

	scannedUsers := []scannedUser{
		{
			roles: []scannedRole{
				{name: "access"},
				{file: "viewer.yaml"},
			},
			loginAs: true,
		},
		{
			roles: []scannedRole{
				{name: "access"},
				{name: "editor"},
			},
		},
	}

	result, err := buildBootstrapState(e2eDir, scannedUsers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state := result.state

	if len(state.Users) != 2 {
		t.Fatalf("got %d users, want 2", len(state.Users))
	}

	if len(state.CustomRoles) != 1 {
		t.Fatalf("got %d custom roles, want 1", len(state.CustomRoles))
	}

	if state.CustomRoles[0].name != "viewer" {
		t.Errorf("custom role name = %q, want %q", state.CustomRoles[0].name, "viewer")
	}

	first := state.Users[0]
	if len(first.Roles) != 2 {
		t.Fatalf("first user has %d roles, want 2", len(first.Roles))
	}

	if first.Roles[0] != "access" {
		t.Errorf("first user role[0] = %q, want %q", first.Roles[0], "access")
	}

	if first.Roles[1] != "viewer" {
		t.Errorf("first user role[1] = %q, want %q", first.Roles[1], "viewer")
	}

	if state.Users[0].Name == "" || state.Users[1].Name == "" {
		t.Error("user has empty generated name")
	}

	if state.Users[0].Name == state.Users[1].Name {
		t.Errorf("users have duplicate names: %s", state.Users[0].Name)
	}

	if len(result.userMapping) != 2 {
		t.Fatalf("got %d user mapping entries, want 2", len(result.userMapping))
	}

	for _, u := range state.Users {
		if _, ok := result.creds[u.Name]; !ok {
			t.Errorf("missing credentials for %s", u.Name)
		}

		if u.PasswordHashBase64 == "" || u.CredentialIDBase64 == "" || u.PublicKeyCBORBase64 == "" {
			t.Errorf("user %s missing webauthn credential fields", u.Name)
		}
	}
}

func TestBuildBootstrapStateDeduplicatesRoles(t *testing.T) {
	e2eDir := t.TempDir()
	rolesDir := createDir(t, e2eDir, "testdata", "roles")
	writeFile(t, rolesDir, "viewer.yaml", viewerRoleYAML)

	scannedUsers := []scannedUser{
		{
			roles: []scannedRole{
				{file: "viewer.yaml"},
			},
		},
		{
			roles: []scannedRole{
				{file: "viewer.yaml"},
			},
		},
	}

	result, err := buildBootstrapState(e2eDir, scannedUsers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.state.CustomRoles) != 1 {
		t.Fatalf("got %d custom roles, want 1 (deduplication failed)", len(result.state.CustomRoles))
	}

	if result.state.CustomRoles[0].name != "viewer" {
		t.Errorf("custom role name = %q, want %q", result.state.CustomRoles[0].name, "viewer")
	}
}

func TestBuildBootstrapStateCustomTraits(t *testing.T) {
	e2eDir := t.TempDir()

	scannedUsers := []scannedUser{
		{
			roles: []scannedRole{{name: "access"}},
			traits: map[string][]string{
				"logins":            {"root", "alice"},
				"kubernetes_groups": {"dev"},
			},
			loginAs: true,
		},
	}

	result, err := buildBootstrapState(e2eDir, scannedUsers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := result.state.Users[0]

	logins := u.Traits["logins"]
	if len(logins) != 2 || logins[0] != "root" || logins[1] != "alice" {
		t.Errorf("logins = %v, want [root alice]", logins)
	}

	groups := u.Traits["kubernetes_groups"]
	if len(groups) != 1 || groups[0] != "dev" {
		t.Errorf("kubernetes_groups = %v, want [dev]", groups)
	}
}

func TestBuildBootstrapStateEmptyRolesError(t *testing.T) {
	e2eDir := t.TempDir()

	users := []scannedUser{{roles: nil, loginAs: true}}
	_, err := buildBootstrapState(e2eDir, users)
	if err == nil {
		t.Fatal("expected error for user with no roles, got nil")
	}

	if !strings.Contains(err.Error(), "no roles") {
		t.Errorf("expected 'no roles' error, got: %v", err)
	}
}

func TestWriteUserMapping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "user-mapping.json")

	mapping := map[string]string{
		`{"roles":["access"]}`:           "brave-falcon",
		`{"roles":["access","editor"]}`: "swift-river",
	}

	if err := writeUserMapping(path, mapping); err != nil {
		t.Fatalf("writeUserMapping: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got) != len(mapping) {
		t.Fatalf("got %d entries, want %d", len(got), len(mapping))
	}

	for k, want := range mapping {
		if got[k] != want {
			t.Errorf("mapping[%q] = %q, want %q", k, got[k], want)
		}
	}
}

func TestWriteCredentialsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "user-credentials.json")

	creds := map[string]*credentials{
		"brave-falcon": {
			password:              "pw-1",
			privateKeyPKCS8Base64: "priv-1",
			credentialIDBase64:    "cid-1",
		},
	}

	if err := writeCredentialsFile(path, creds); err != nil {
		t.Fatalf("writeCredentialsFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	// The credentials file holds password hashes and WebAuthn private keys,
	// so restricted (0o600) perms are a load-bearing invariant.
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %o, want 0o600", perm)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var got map[string]credentialsJSON
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	entry, ok := got["brave-falcon"]
	if !ok {
		t.Fatalf("missing entry for brave-falcon: %+v", got)
	}

	if entry.Password != "pw-1" {
		t.Errorf("password = %q, want %q", entry.Password, "pw-1")
	}

	if entry.WebauthnPrivateKey != "priv-1" {
		t.Errorf("webauthnPrivateKey = %q, want %q", entry.WebauthnPrivateKey, "priv-1")
	}

	if entry.WebauthnCredentialId != "cid-1" {
		t.Errorf("webauthnCredentialId = %q, want %q", entry.WebauthnCredentialId, "cid-1")
	}
}
