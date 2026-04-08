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

	// Verify first user has roles ["access", "viewer"].
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

	// Verify generated names are human-readable and unique.
	if state.Users[0].Name == "" {
		t.Error("first user has empty generated name")
	}

	if state.Users[1].Name == "" {
		t.Error("second user has empty generated name")
	}

	if state.Users[0].Name == state.Users[1].Name {
		t.Errorf("users have duplicate names: %s", state.Users[0].Name)
	}

	// Verify both users have credentials.
	for _, u := range state.Users {
		if _, ok := result.creds[u.Name]; !ok {
			t.Errorf("missing credentials for %s", u.Name)
		}
	}

	// Verify user mapping contains entries for both users.
	if len(result.userMapping) != 2 {
		t.Fatalf("got %d user mapping entries, want 2", len(result.userMapping))
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

	// Verify default traits (logins: [root]) when no traits specified.
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
