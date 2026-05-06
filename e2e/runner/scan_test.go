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
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gravitational/teleport/e2e/runner/fixtures"
)

func TestScanFile(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantNames []string
	}{
		{
			name:      "test.use with single fixture",
			content:   `test.use({ fixtures: ['ssh-node'] });`,
			wantNames: []string{"ssh-node"},
		},
		{
			name:      "test.use with multiple fixtures",
			content:   `test.use({ fixtures: ['ssh-node', 'connect'] });`,
			wantNames: []string{"ssh-node", "connect"},
		},
		{
			name:      "test.use with nested brackets",
			content:   `test.use({ fixtures: [['connect'], { option: true }] });`,
			wantNames: []string{"connect"},
		},
		{
			name:      "bare fixtures array without test.use is ignored",
			content:   `  fixtures: [['connect'], { option: true }],`,
			wantNames: nil,
		},
		{
			name:      "commented out line is skipped",
			content:   `// test.use({ fixtures: ['ssh-node'] });`,
			wantNames: nil,
		},
		{
			name:      "no fixtures",
			content:   `test.use({ autoLogin: true });`,
			wantNames: nil,
		},
		{
			name:      "mixed with other options",
			content:   `test.use({ autoLogin: true, fixtures: ['connect'] });`,
			wantNames: []string{"connect"},
		},
		{
			name: "multiline fixture array",
			content: `test.use({
  fixtures: [
    'ssh-node',
    'connect',
  ],
});`,
			wantNames: []string{"ssh-node", "connect"},
		},
		{
			name: "multiline with comments between",
			content: `test.use({
  fixtures: [
    // 'ssh-node',
    'connect',
  ],
});`,
			wantNames: []string{"connect"},
		},
		{
			name:      "nested parens in options before fixtures",
			content:   `test.use({ timeout: getTimeout(), fixtures: ['ssh-node'] });`,
			wantNames: []string{"ssh-node"},
		},
		{
			name: "block comment is stripped",
			content: `/* test.use({ fixtures: ['ssh-node'] }); */
test.use({ fixtures: ['connect'] });`,
			wantNames: []string{"connect"},
		},
		{
			name:      "trailing inline comment is stripped",
			content:   `someCode; // test.use({ fixtures: ['connect'] })`,
			wantNames: nil,
		},
		{
			name:      "inline comment after real fixture is stripped",
			content:   `test.use({ fixtures: ['ssh-node'] }); // test.use({ fixtures: ['connect'] })`,
			wantNames: []string{"ssh-node"},
		},
		{
			name:      "braces inside string literals do not corrupt blocks",
			content:   "const s = \"{ not a block }\";\ntest.use({ fixtures: ['ssh-node'] });",
			wantNames: []string{"ssh-node"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tmpFile := filepath.Join(dir, "test.spec.ts")
			writeFile(t, dir, "test.spec.ts", tt.content)

			got := scanFile(tmpFile, 0)

			if len(got) != len(tt.wantNames) {
				t.Fatalf("got %d fixtures, want %d", len(got), len(tt.wantNames))
			}

			for i, f := range got {
				if f.Name != tt.wantNames[i] {
					t.Errorf("fixture[%d] name = %q, want %q", i, f.Name, tt.wantNames[i])
				}
			}
		})
	}
}

func TestScanFileLineScope(t *testing.T) {
	content := `test.use({ fixtures: ['ssh-node'] });       // 1  (top-level)
                                                            // 2
test.describe('connect tests', () => {                      // 3
  test.use({ fixtures: ['connect'] });                      // 4
                                                            // 5
  test('opens connect', async () => {                       // 6
    // test body                                            // 7
  });                                                       // 8
});                                                         // 9
                                                            // 10
test.describe('web tests', () => {                          // 11
  test('opens web', async () => {                           // 12
    // test body                                            // 13
  });                                                       // 14
}); 																											  // 15
                                                            // 16
test.describe(() => {                                       // 17
  test.use({ fixtures: ['connect'] });                      // 18
  test('one', async () => {                                 // 19
    // test body                                            // 20
  });                                                       // 21
  test('two', async () => {                                 // 22
    // test body                                            // 23
  });                                                       // 24
});                                                         // 25`

	tests := []struct {
		name       string
		targetLine int
		wantNames  []string
	}{
		{
			name:       "no line filter returns all fixtures",
			targetLine: 0,
			wantNames:  []string{"ssh-node", "connect", "connect"},
		},
		{
			name:       "line inside connect describe gets top-level and connect",
			targetLine: 7,
			wantNames:  []string{"ssh-node", "connect"},
		},
		{
			name:       "line inside web describe gets only top-level",
			targetLine: 13,
			wantNames:  []string{"ssh-node"},
		},
		{
			name:       "line targeting specific test inside describe with test.use",
			targetLine: 23,
			wantNames:  []string{"ssh-node", "connect"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tmpFile := filepath.Join(dir, "test.spec.ts")
			writeFile(t, dir, "test.spec.ts", content)

			got := scanFile(tmpFile, tt.targetLine)

			if len(got) != len(tt.wantNames) {
				t.Fatalf("got %d fixtures, want %d: %v", len(got), len(tt.wantNames), fixtureNames(got))
			}

			for i, f := range got {
				if f.Name != tt.wantNames[i] {
					t.Errorf("fixture[%d] name = %q, want %q", i, f.Name, tt.wantNames[i])
				}
			}
		})
	}
}

func TestParseHelperImports(t *testing.T) {
	content := `import { test, expect } from '@gravitational/e2e/helpers/connect';
import { startUrl } from '@gravitational/e2e/helpers/env';
import { chromium } from '@playwright/test';
`
	tmpFile := filepath.Join(t.TempDir(), "test.spec.ts")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := parseHelperImports(tmpFile)
	want := []string{"connect", "env"}

	if len(got) != len(want) {
		t.Fatalf("got %d imports, want %d: %v", len(got), len(want), got)
	}

	for i, h := range got {
		if h != want[i] {
			t.Errorf("import[%d] = %q, want %q", i, h, want[i])
		}
	}
}

func TestScanFixtures(t *testing.T) {
	e2eDir := t.TempDir()

	helpersDir := createDir(t, e2eDir, "helpers")
	testsDir := createDir(t, e2eDir, "tests", "connect")
	webDir := createDir(t, e2eDir, "tests", "web", "authenticated")

	// Helper that declares Connect fixture as default.
	connectHelper := `import { test as fixtureBase } from './fixtures';
export const test = fixtureBase.extend<{}>({});
test.use({ fixtures: ['connect'] });
`
	writeFile(t, helpersDir, "connect.ts", connectHelper)

	// Helper with no fixtures.
	testHelper := `import { test as base } from './fixtures';
export const test = base.extend<{}>({});
`
	writeFile(t, helpersDir, "test.ts", testHelper)

	// Connect spec file imports from connect helper.
	connectSpec := `import { test } from '@gravitational/e2e/helpers/connect';
test('something', async () => {});
`
	writeFile(t, testsDir, "auth.spec.ts", connectSpec)

	// Web spec file imports from test helper (no fixtures).
	webSpec := `import { test } from '@gravitational/e2e/helpers/test';
test('something', async () => {});
`
	writeFile(t, webDir, "roles.spec.ts", webSpec)

	t.Run("connect test detects Connect fixture via helper", func(t *testing.T) {
		rel, _ := filepath.Rel(e2eDir, filepath.Join(testsDir, "auth.spec.ts"))
		targets, err := resolveTargetsWithHelpers(e2eDir, []string{rel})
		if err != nil {
			t.Fatal(err)
		}
		got := scanFixturesFromTargets(targets)

		if len(got) != 1 {
			t.Fatalf("expected 1 fixture, got %d", len(got))
		}

		if got[0].Name != "connect" {
			t.Errorf("expected fixture name 'connect', got %q", got[0].Name)
		}
	})

	t.Run("web test does not detect Connect fixture", func(t *testing.T) {
		rel, _ := filepath.Rel(e2eDir, filepath.Join(webDir, "roles.spec.ts"))
		targets, err := resolveTargetsWithHelpers(e2eDir, []string{rel})
		if err != nil {
			t.Fatal(err)
		}
		got := scanFixturesFromTargets(targets)

		if len(got) != 0 {
			t.Fatalf("expected 0 fixtures, got %d", len(got))
		}
	})
}

func TestResolveFilesToScan(t *testing.T) {
	e2eDir := t.TempDir()
	testsDir := createDir(t, e2eDir, "tests", "connect")

	writeFile(t, testsDir, "auth.spec.ts", "test('auth', async () => {});")
	writeFile(t, testsDir, "session.spec.ts", "test('session', async () => {});")

	t.Run("file with line number", func(t *testing.T) {
		rel, _ := filepath.Rel(e2eDir, filepath.Join(testsDir, "auth.spec.ts"))
		targets, err := resolveFilesToScan(e2eDir, []string{rel + ":42"})
		if err != nil {
			t.Fatal(err)
		}

		if len(targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(targets))
		}

		if targets[0].line != 42 {
			t.Errorf("expected line 42, got %d", targets[0].line)
		}
	})

	t.Run("directory expands to spec files", func(t *testing.T) {
		targets, err := resolveFilesToScan(e2eDir, []string{"tests/connect"})
		if err != nil {
			t.Fatal(err)
		}

		if len(targets) != 2 {
			t.Fatalf("expected 2 targets, got %d", len(targets))
		}

		for _, tgt := range targets {
			if tgt.line != 0 {
				t.Errorf("directory target should have line=0, got %d", tgt.line)
			}
		}
	})

	t.Run("substring filter matches spec files", func(t *testing.T) {
		targets, err := resolveFilesToScan(e2eDir, []string{"auth"})
		if err != nil {
			t.Fatal(err)
		}

		if len(targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(targets))
		}
	})
}

func TestScanUsers(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantUsers []scannedUser
	}{
		{
			name: "singular user with built-in roles",
			content: `test.use({
  user: { roles: ['access', 'editor'] },
});`,
			wantUsers: []scannedUser{
				{
					roles: []scannedRole{
						{name: "access"},
						{name: "editor"},
					},
					loginAs: true,
				},
			},
		},
		{
			name: "users array with loginAs",
			content: `test.use({
  users: [
    { roles: ['access', 'editor'], loginAs: true },
    { roles: [{ file: '@gravitational/e2e/roles/viewer.yaml' }] },
  ],
});`,
			wantUsers: []scannedUser{
				{
					roles: []scannedRole{
						{name: "access"},
						{name: "editor"},
					},
					loginAs: true,
				},
				{
					roles: []scannedRole{
						{file: "viewer.yaml"},
					},
				},
			},
		},
		{
			name: "user with file role",
			content: `test.use({
  user: { roles: [{ file: '@gravitational/e2e/roles/viewer.yaml' }] },
});`,
			wantUsers: []scannedUser{
				{
					roles: []scannedRole{
						{file: "viewer.yaml"},
					},
					loginAs: true,
				},
			},
		},
		{
			name: "user with traits",
			content: `test.use({
  user: {
    roles: ['access'],
    traits: { logins: ['root', 'alice'], kubernetes_groups: ['dev'] },
  },
});`,
			wantUsers: []scannedUser{
				{
					roles: []scannedRole{
						{name: "access"},
					},
					traits: map[string][]string{
						"logins":            {"root", "alice"},
						"kubernetes_groups": {"dev"},
					},
					loginAs: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tmpFile := filepath.Join(dir, "test.spec.ts")
			writeFile(t, dir, "test.spec.ts", tt.content)

			got, err := scanFileUsers(tmpFile, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.wantUsers) {
				t.Fatalf("got %d users, want %d", len(got), len(tt.wantUsers))
			}

			for i, u := range got {
				want := tt.wantUsers[i]

				if u.loginAs != want.loginAs {
					t.Errorf("user[%d] loginAs = %v, want %v", i, u.loginAs, want.loginAs)
				}

				if len(u.roles) != len(want.roles) {
					t.Fatalf("user[%d] got %d roles, want %d", i, len(u.roles), len(want.roles))
				}

				for j, r := range u.roles {
					if r.name != want.roles[j].name {
						t.Errorf("user[%d] role[%d] name = %q, want %q", i, j, r.name, want.roles[j].name)
					}

					if r.file != want.roles[j].file {
						t.Errorf("user[%d] role[%d] file = %q, want %q", i, j, r.file, want.roles[j].file)
					}
				}

				if want.traits != nil {
					if u.traits == nil {
						t.Errorf("user[%d] traits = nil, want %v", i, want.traits)
					} else {
						for k, wantVals := range want.traits {
							gotVals, ok := u.traits[k]
							if !ok {
								t.Errorf("user[%d] missing trait %q", i, k)
								continue
							}

							if len(gotVals) != len(wantVals) {
								t.Errorf("user[%d] trait %q has %d values, want %d", i, k, len(gotVals), len(wantVals))
								continue
							}

							for vi, v := range gotVals {
								if v != wantVals[vi] {
									t.Errorf("user[%d] trait %q[%d] = %q, want %q", i, k, vi, v, wantVals[vi])
								}
							}
						}
					}
				}
			}
		})
	}
}

func TestScanUsersDefaultUser(t *testing.T) {
	e2eDir := t.TempDir()
	testsDir := createDir(t, e2eDir, "tests")

	// A spec that declares no users at all.
	writeFile(t, testsDir, "basic.spec.ts", `test.use({ fixtures: ['ssh-node'] });`)

	targets, err := resolveTargetsWithHelpers(e2eDir, []string{"tests/basic.spec.ts"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := scanUsersFromTargets(targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 user, got %d", len(got))
	}

	if len(got[0].roles) != 2 {
		t.Fatalf("expected 2 roles for default user, got %d", len(got[0].roles))
	}

	if got[0].roles[0].name != "access" {
		t.Errorf("expected first role 'access', got %q", got[0].roles[0].name)
	}

	if got[0].roles[1].name != "editor" {
		t.Errorf("expected second role 'editor', got %q", got[0].roles[1].name)
	}

	if !got[0].loginAs {
		t.Error("expected default user to have loginAs=true")
	}
}

func TestScanUsersMutuallyExclusiveError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.spec.ts", `test.use({
  user: { roles: ['access'] },
  users: [{ roles: ['access'] }],
});`)

	_, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0)
	if err == nil {
		t.Fatal("expected error for user + users in same test.use(), got nil")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutual-exclusivity error, got: %v", err)
	}
}

func TestScanUsersMultipleLoginAsError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.spec.ts", `test.use({
  users: [
    { roles: ['access'], loginAs: true },
    { roles: ['editor'], loginAs: true },
  ],
});`)

	_, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0)
	if err == nil {
		t.Fatal("expected error for multiple loginAs: true, got nil")
	}

	if !strings.Contains(err.Error(), "loginAs") {
		t.Errorf("expected loginAs error, got: %v", err)
	}
}

func TestScanUsersDuplicateRoleWarn(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantWarns []string
		wantNone  bool
	}{
		{
			name: "duplicate built-in roles warn",
			content: `test.use({
  user: { roles: ['access', 'access', 'editor'] },
});`,
			wantWarns: []string{"role=access"},
		},
		{
			name: "duplicate file roles warn",
			content: `test.use({
  user: { roles: [{ file: '@gravitational/e2e/roles/viewer.yaml' }, { file: '@gravitational/e2e/roles/viewer.yaml' }] },
});`,
			wantWarns: []string{"role=file:viewer.yaml"},
		},
		{
			name: "duplicate triggers per-array-entry warn",
			content: `test.use({
  users: [
    { roles: ['access', 'access'] },
    { roles: ['editor', 'editor'], loginAs: true },
  ],
});`,
			wantWarns: []string{"role=access", "role=editor"},
		},
		{
			name: "no duplicates produces no warn",
			content: `test.use({
  user: { roles: ['access', 'editor'] },
});`,
			wantNone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
			t.Cleanup(func() { slog.SetDefault(prev) })

			dir := t.TempDir()
			writeFile(t, dir, "test.spec.ts", tt.content)

			if _, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			out := buf.String()
			if tt.wantNone {
				if strings.Contains(out, "duplicate role") {
					t.Errorf("expected no duplicate-role warns, got: %s", out)
				}
				return
			}

			for _, want := range tt.wantWarns {
				if !strings.Contains(out, want) {
					t.Errorf("missing warn substring %q in output:\n%s", want, out)
				}
			}
		})
	}
}

func TestScanUsersDelimitersInStringLiterals(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantRoles []string
	}{
		{
			name: "brace inside trait string does not close user block early",
			content: `test.use({
  user: {
    roles: ['access'],
    traits: { logins: ['has}brace', 'ok'] },
  },
});`,
			wantRoles: []string{"access"},
		},
		{
			name: "bracket inside trait string does not close users array early",
			content: `test.use({
  users: [
    { roles: ['access'], traits: { logins: ['has]bracket'] } },
    { roles: ['editor'], loginAs: true },
  ],
});`,
			wantRoles: []string{"access", "editor"},
		},
		{
			name: "backtick template with delimiters does not corrupt parsing",
			content: "test.use({\n  user: {\n    roles: ['access'],\n    traits: { logins: [`weird}{`] },\n  },\n});",
			wantRoles: []string{"access"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "test.spec.ts", tt.content)

			got, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.wantRoles) {
				t.Fatalf("got %d users, want %d: %+v", len(got), len(tt.wantRoles), got)
			}

			for i, want := range tt.wantRoles {
				if len(got[i].roles) != 1 || got[i].roles[0].name != want {
					t.Errorf("users[%d].roles = %+v, want [{name: %q}]", i, got[i].roles, want)
				}
			}
		})
	}
}

func TestScanUsersIdentifierBoundaries(t *testing.T) {
	// Declarations that look like they should match the user/users/roles/traits
	// regexes but don't because the identifier has additional word characters.
	dir := t.TempDir()
	writeFile(t, dir, "test.spec.ts", `test.use({
  fixtures: ['ssh-node'],
  super_user: { roles: ['nope'] },
  customUsers: [{ roles: ['nope'] }],
  myRoles: ['nope'],
});`)

	got, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("expected 0 users (identifiers should not match), got %d: %+v", len(got), got)
	}
}

func createDir(t *testing.T, path ...string) string {
	t.Helper()

	dir := filepath.Join(path...)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("creating directory %s: %v", dir, err)
	}

	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file %s: %v", path, err)
	}
}

func fixtureNames(ff []*fixtures.Fixture) []string {
	names := make([]string, len(ff))
	for i, f := range ff {
		names[i] = f.Name
	}
	return names
}
