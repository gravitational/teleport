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

func TestBuildBootstrapState(t *testing.T) {
	e2eDir := t.TempDir()
	rolesDir := createDir(t, e2eDir, "testdata", "roles")
	writeFile(t, rolesDir, "viewer.yaml", viewerRoleYAML)

	scannedUsers := []scannedUser{
		{
			name: "alice",
			roles: []scannedRole{
				{name: "access"},
				{file: "viewer.yaml"},
			},
		},
		{
			name: "bob",
			roles: []scannedRole{
				{name: "access"},
				{name: "editor"},
			},
		},
	}

	state, creds, err := buildBootstrapState(e2eDir, scannedUsers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify 2 users.
	if len(state.Users) != 2 {
		t.Fatalf("got %d users, want 2", len(state.Users))
	}

	// Verify 1 custom role named "viewer".
	if len(state.CustomRoles) != 1 {
		t.Fatalf("got %d custom roles, want 1", len(state.CustomRoles))
	}

	if state.CustomRoles[0].name != "viewer" {
		t.Errorf("custom role name = %q, want %q", state.CustomRoles[0].name, "viewer")
	}

	// Verify alice has roles ["access", "viewer"].
	alice := state.Users[0]
	if alice.Name != "alice" {
		t.Fatalf("first user name = %q, want %q", alice.Name, "alice")
	}

	if len(alice.Roles) != 2 {
		t.Fatalf("alice has %d roles, want 2", len(alice.Roles))
	}

	if alice.Roles[0] != "access" {
		t.Errorf("alice role[0] = %q, want %q", alice.Roles[0], "access")
	}

	if alice.Roles[1] != "viewer" {
		t.Errorf("alice role[1] = %q, want %q", alice.Roles[1], "viewer")
	}

	// Verify both users have credentials.
	if _, ok := creds["alice"]; !ok {
		t.Error("missing credentials for alice")
	}

	if _, ok := creds["bob"]; !ok {
		t.Error("missing credentials for bob")
	}

	// Verify bootstrap users have non-empty credential fields.
	for _, u := range state.Users {
		if u.PasswordHashBase64 == "" {
			t.Errorf("user %s has empty PasswordHashBase64", u.Name)
		}

		if u.CredentialIDBase64 == "" {
			t.Errorf("user %s has empty CredentialIDBase64", u.Name)
		}

		if u.PublicKeyCBORBase64 == "" {
			t.Errorf("user %s has empty PublicKeyCBORBase64", u.Name)
		}
	}

	// Verify traits.
	for _, u := range state.Users {
		logins, ok := u.Traits["logins"]
		if !ok {
			t.Errorf("user %s missing logins trait", u.Name)
		} else if len(logins) != 1 || logins[0] != "root" {
			t.Errorf("user %s logins = %v, want [root]", u.Name, logins)
		}
	}
}

func TestBuildBootstrapStateDeduplicatesRoles(t *testing.T) {
	e2eDir := t.TempDir()
	rolesDir := createDir(t, e2eDir, "testdata", "roles")
	writeFile(t, rolesDir, "viewer.yaml", viewerRoleYAML)

	scannedUsers := []scannedUser{
		{
			name: "alice",
			roles: []scannedRole{
				{file: "viewer.yaml"},
			},
		},
		{
			name: "bob",
			roles: []scannedRole{
				{file: "viewer.yaml"},
			},
		},
	}

	state, _, err := buildBootstrapState(e2eDir, scannedUsers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(state.CustomRoles) != 1 {
		t.Fatalf("got %d custom roles, want 1 (deduplication failed)", len(state.CustomRoles))
	}

	if state.CustomRoles[0].name != "viewer" {
		t.Errorf("custom role name = %q, want %q", state.CustomRoles[0].name, "viewer")
	}
}
