package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

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
import { test as entTest } from '@gravitational/e-e2e/helpers/test';
import { chromium } from '@playwright/test';
`
	tmpFile := filepath.Join(t.TempDir(), "test.spec.ts")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Imports keep their package so the helper resolves against the right tree.
	got := parseHelperImports(tmpFile)
	want := []string{"e2e/connect", "e2e/env", "e-e2e/test"}

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
		targets, err := resolveTargetsWithHelpers(suiteDirs{shared: e2eDir, suite: e2eDir}, []string{rel})
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
		targets, err := resolveTargetsWithHelpers(suiteDirs{shared: e2eDir, suite: e2eDir}, []string{rel})
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

			got, err := scanFileUsers(tmpFile, 0, "test.spec.ts")
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

	targets, err := resolveTargetsWithHelpers(suiteDirs{shared: e2eDir, suite: e2eDir}, []string{"tests/basic.spec.ts"})
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

func TestScanRecordings(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		wantUsers      int
		wantRecordings []string // recordings on the first user
		wantLoginAs    bool
	}{
		{
			name: "top-level recordings creates default user",
			content: `test.use({
  recordings: ['ssh-session-1', 'desktop-session-2'],
});`,
			wantUsers:      1,
			wantRecordings: []string{"ssh-session-1", "desktop-session-2"},
			wantLoginAs:    true,
		},
		{
			name: "recordings on user definition",
			content: `test.use({
  user: { roles: ['access'], recordings: ['ssh-session-1'] },
});`,
			wantUsers:      1,
			wantRecordings: []string{"ssh-session-1"},
			wantLoginAs:    true,
		},
		{
			name:           "no recordings",
			content:        `test.use({ user: { roles: ['access'] } });`,
			wantUsers:      1,
			wantRecordings: nil,
			wantLoginAs:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tmpFile := filepath.Join(dir, "test.spec.ts")
			writeFile(t, dir, "test.spec.ts", tt.content)

			got, err := scanFileUsers(tmpFile, 0, "test.spec.ts")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != tt.wantUsers {
				t.Fatalf("got %d users, want %d", len(got), tt.wantUsers)
			}

			if tt.wantUsers == 0 {
				return
			}

			if got[0].loginAs != tt.wantLoginAs {
				t.Errorf("loginAs = %v, want %v", got[0].loginAs, tt.wantLoginAs)
			}

			if len(got[0].recordings) != len(tt.wantRecordings) {
				t.Fatalf("got %d recordings, want %d: %v", len(got[0].recordings), len(tt.wantRecordings), got[0].recordings)
			}

			for i, r := range got[0].recordings {
				if r != tt.wantRecordings[i] {
					t.Errorf("recording[%d] = %q, want %q", i, r, tt.wantRecordings[i])
				}
			}
		})
	}
}

func TestScanFileHelperRejectsUserAndRecordings(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"user", `test.use({ user: { roles: ['access'] } });`},
		{"users", `test.use({ users: [{ roles: ['access'], loginAs: true }] });`},
		{"recordings", `test.use({ recordings: ['ssh-1'] });`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "helper.ts", tc.content)

			_, err := scanFileUsers(filepath.Join(dir, "helper.ts"), 0, "")
			if err == nil {
				t.Fatal("expected error for helper-declared user/users/recordings, got nil")
			}

			if !strings.Contains(err.Error(), "helper modules cannot declare") {
				t.Errorf("expected helper-rejection error, got: %v", err)
			}
		})
	}
}

func TestScanTopLevelRecordingsSynthIsDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "spec.ts", `test.use({ recordings: ['ssh-1'] });`)

	got, err := scanFileUsers(filepath.Join(dir, "spec.ts"), 0, "tests/spec.ts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d users, want 1", len(got))
	}

	if !got[0].isDefault {
		t.Errorf("isDefault = false, want true")
	}

	if got[0].sourceFile != "" {
		t.Errorf("sourceFile = %q, want empty", got[0].sourceFile)
	}
}

func TestScanUsersMutuallyExclusiveError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.spec.ts", `test.use({
  user: { roles: ['access'] },
  users: [{ roles: ['access'] }],
});`)

	_, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts")
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

	_, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts")
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

			if _, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts"); err != nil {
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

func TestScanRecordingsAfterUserBlock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.spec.ts", `test.use({
  user: { roles: ['access'], recordings: ['nested'] },
  recordings: ['top-level'],
});`)

	_, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts")
	if err == nil {
		t.Fatal("expected mutual-exclusivity error, got nil")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutual-exclusivity error mentioning recordings, got: %v", err)
	}
}

func TestHasTopLevelRecordingsIgnoresNested(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"top-level only", `test.use({ recordings: ['a'] })`, true},
		{"nested in user", `test.use({ user: { roles: ['x'], recordings: ['a'] } })`, false},
		{"nested in arbitrary fixture", `test.use({ traits: { recordings: ['a'] } })`, false},
		{"sibling top-level wins over nested", `test.use({ traits: { recordings: ['a'] }, recordings: ['b'] })`, true},
		{"recordings inside string literal is not a key", `test.use({ note: 'recordings: [foo]' })`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := findTestUseCalls(tc.body)
			if len(calls) != 1 {
				t.Fatalf("findTestUseCalls returned %d calls, want 1", len(calls))
			}

			body := tc.body[calls[0].start:calls[0].end]
			if got := hasTopLevelRecordings(body); got != tc.want {
				t.Errorf("hasTopLevelRecordings = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestScanFileTopLevelRecordingsAcrossSiblingDescribesAllowed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.spec.ts", `test.describe('A', () => {
  test.use({ user: { roles: ['access'] } });
});

test.describe('B', () => {
  test.use({ recordings: ['ssh-1'] });
});`)

	got, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts")
	if err != nil {
		t.Fatalf("unexpected error for sibling-describe scopes: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d users, want 2 (one per describe)", len(got))
	}
}

func TestScanFileTopLevelRecordingsAcrossOneLineSiblingDescribesAllowed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.spec.ts", `test.describe('A', () => { test.use({ user: { roles: ['access'] } }); });
test.describe('B', () => { test.use({ recordings: ['ssh-1'] }); });`)

	got, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts")
	if err != nil {
		t.Fatalf("unexpected error for one-line sibling-describe scopes: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d users, want 2 (one per describe)", len(got))
	}
}

func TestScanFileTopLevelRecordingsOneLineNestedDescribeRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.spec.ts", `test.use({ recordings: ['ssh-1'] });
test.describe('inner', () => { test.use({ user: { roles: ['access'] } }); });`)

	_, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts")
	if err == nil {
		t.Fatal("expected error for file-scope recordings + one-line describe user, got nil")
	}

	if !strings.Contains(err.Error(), "top-level recordings") {
		t.Errorf("expected top-level-recordings error, got: %v", err)
	}
}

func TestScanFileTopLevelRecordingsNestedDescribeRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.spec.ts", `test.use({ recordings: ['ssh-1'] });

test.describe('inner', () => {
  test.use({ user: { roles: ['access'] } });
});`)

	_, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts")
	if err == nil {
		t.Fatal("expected error for file-scope recordings + describe-scope user, got nil")
	}

	if !strings.Contains(err.Error(), "top-level recordings") {
		t.Errorf("expected top-level-recordings error, got: %v", err)
	}
}

func TestScanFileTopLevelRecordingsWithExplicitUserError(t *testing.T) {
	// Playwright merges multiple file-scope test.use() calls; mixing a
	// top-level recordings call with a user/users call leaves the recordings
	// orphaned on a synthetic default user. Scanner must reject this.
	cases := []struct {
		name    string
		content string
	}{
		{
			name: "user then top-level recordings",
			content: `test.use({ user: { roles: ['access'] } });
test.use({ recordings: ['ssh-1'] });`,
		},
		{
			name: "top-level recordings then users",
			content: `test.use({ recordings: ['ssh-1'] });
test.use({ users: [{ roles: ['access'], loginAs: true }] });`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "test.spec.ts", tc.content)

			_, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts")
			if err == nil {
				t.Fatal("expected error mixing top-level recordings with explicit user/users, got nil")
			}

			if !strings.Contains(err.Error(), "top-level recordings") {
				t.Errorf("expected top-level-recordings error, got: %v", err)
			}
		})
	}
}

func TestScanUsersNestedUserFixturesIgnored(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantUsers int
	}{
		{
			name: "user nested inside another fixture object is not detected",
			content: `test.use({
  customFixture: { user: { roles: ['nope'] } },
  recordings: ['ssh-1'],
});`,
			wantUsers: 1,
		},
		{
			name: "users nested inside another fixture is not detected",
			content: `test.use({
  customFixture: { users: [{ roles: ['nope'] }] },
  recordings: ['ssh-1'],
});`,
			wantUsers: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "test.spec.ts", tt.content)

			got, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != tt.wantUsers {
				t.Fatalf("got %d users, want %d: %+v", len(got), tt.wantUsers, got)
			}
		})
	}
}

func TestScanUsersUserFieldsAreDepth0Only(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		wantRecordings []string
		wantRoleNames  []string
	}{
		{
			name: "nested recordings inside traits ignored when no top-level recordings",
			content: `test.use({
  user: {
    roles: ['access'],
    traits: { recordings: ['nested-fake'] },
  },
});`,
			wantRoleNames: []string{"access"},
		},
		{
			name: "nested recordings inside traits do not shadow top-level recordings",
			content: `test.use({
  user: {
    roles: ['access'],
    traits: { recordings: ['nested-fake'] },
    recordings: ['real-1', 'real-2'],
  },
});`,
			wantRecordings: []string{"real-1", "real-2"},
			wantRoleNames:  []string{"access"},
		},
		{
			name: "nested roles inside traits do not shadow top-level roles",
			content: `test.use({
  user: {
    traits: { roles: ['nested-fake'] },
    roles: ['access', 'editor'],
  },
});`,
			wantRoleNames: []string{"access", "editor"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "test.spec.ts", tt.content)

			got, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != 1 {
				t.Fatalf("got %d users, want 1: %+v", len(got), got)
			}

			if len(got[0].recordings) != len(tt.wantRecordings) {
				t.Fatalf("got %d recordings, want %d: %v", len(got[0].recordings), len(tt.wantRecordings), got[0].recordings)
			}

			for i, r := range got[0].recordings {
				if r != tt.wantRecordings[i] {
					t.Errorf("recording[%d] = %q, want %q", i, r, tt.wantRecordings[i])
				}
			}

			if len(got[0].roles) != len(tt.wantRoleNames) {
				t.Fatalf("got %d roles, want %d: %+v", len(got[0].roles), len(tt.wantRoleNames), got[0].roles)
			}

			for i, r := range got[0].roles {
				if r.name != tt.wantRoleNames[i] {
					t.Errorf("role[%d] name = %q, want %q", i, r.name, tt.wantRoleNames[i])
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
			name:      "backtick template with delimiters does not corrupt parsing",
			content:   "test.use({\n  user: {\n    roles: ['access'],\n    traits: { logins: [`weird}{`] },\n  },\n});",
			wantRoles: []string{"access"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "test.spec.ts", tt.content)

			got, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts")
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

func TestScanUsersQuotedTraitKeys(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantTraits map[string][]string
	}{
		{
			name: "single-quoted trait key with hyphen",
			content: `test.use({
  user: {
    roles: ['access'],
    traits: { 'tenant-id': ['acme', 'globex'] },
  },
});`,
			wantTraits: map[string][]string{"tenant-id": {"acme", "globex"}},
		},
		{
			name: "double-quoted trait key",
			content: `test.use({
  user: {
    roles: ['access'],
    traits: { "db-roles": ['reader'] },
  },
});`,
			wantTraits: map[string][]string{"db-roles": {"reader"}},
		},
		{
			name: "mixed quoted and bare trait keys",
			content: `test.use({
  user: {
    roles: ['access'],
    traits: { logins: ['root'], 'tenant-id': ['acme'] },
  },
});`,
			wantTraits: map[string][]string{
				"logins":    {"root"},
				"tenant-id": {"acme"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "test.spec.ts", tt.content)

			got, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != 1 {
				t.Fatalf("got %d users, want 1", len(got))
			}

			if len(got[0].traits) != len(tt.wantTraits) {
				t.Fatalf("got %d traits, want %d: %+v", len(got[0].traits), len(tt.wantTraits), got[0].traits)
			}

			for k, want := range tt.wantTraits {
				gotVals, ok := got[0].traits[k]
				if !ok {
					t.Errorf("missing trait %q", k)
					continue
				}
				if len(gotVals) != len(want) {
					t.Errorf("trait %q: got %d values, want %d", k, len(gotVals), len(want))
					continue
				}
				for i, v := range gotVals {
					if v != want[i] {
						t.Errorf("trait %q[%d] = %q, want %q", k, i, v, want[i])
					}
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

	got, err := scanFileUsers(filepath.Join(dir, "test.spec.ts"), 0, "test.spec.ts")
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

func TestExtractTeleportConfigs(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []scopedTeleportConfig
		wantErr bool
	}{
		{
			name:    "no declaration",
			content: `test.use({ user: { roles: ['access'] } });`,
			want:    nil,
		},
		{
			name: "describe-scoped config carries its line",
			content: `test.describe('grp', () => {
  test.use({ teleport: { config: { a: 1 } } });
  test('x', () => {});
});`,
			want: []scopedTeleportConfig{{raw: `{ a: 1 }`, line: 1}},
		},
		{
			name: "two describes with different configs",
			content: `test.describe('one', () => {
  test.use({ teleport: { config: { a: 1 } } });
});
test.describe('two', () => {
  test.use({ teleport: { config: { a: 2 } } });
});`,
			want: []scopedTeleportConfig{
				{raw: `{ a: 1 }`, line: 1},
				{raw: `{ a: 2 }`, line: 4},
			},
		},
		{
			name:    "file-level config errors",
			content: `test.use({ teleport: { config: { a: 1 } } });`,
			wantErr: true,
		},
		{
			name: "conflicting configs in one describe errors",
			content: `test.describe('grp', () => {
  test.use({ teleport: { config: { a: 1 } } });
  test.use({ teleport: { config: { a: 2 } } });
});`,
			wantErr: true,
		},
		{
			name:    "teleport without config errors",
			content: `test.use({ teleport: { foo: 1 } });`,
			wantErr: true,
		},
		{
			name: "env alongside config",
			content: `test.describe('grp', () => {
  test.use({ teleport: { config: { a: 1 }, env: { FOO: 'bar' } } });
});`,
			want: []scopedTeleportConfig{
				{raw: `{ a: 1 }`, line: 1, env: map[string]string{"FOO": "bar"}},
			},
		},
		{
			name: "malformed env errors",
			content: `test.describe('grp', () => {
  test.use({ teleport: { config: { a: 1 }, env: { FOO: { nested: 1 } } } });
});`,
			wantErr: true,
		},
		{
			name: "license without config",
			content: `test.describe('grp', () => {
  test.use({ teleport: { license: 'featurehiding' } });
});`,
			want: []scopedTeleportConfig{
				{raw: "", license: "featurehiding", line: 1},
			},
		},
		{
			name: "license alongside config and env",
			content: `test.describe('grp', () => {
  test.use({ teleport: { license: 'cloud-ent-staging', config: { a: 1 }, env: { FOO: 'bar' } } });
});`,
			want: []scopedTeleportConfig{
				{raw: `{ a: 1 }`, license: "cloud-ent-staging", line: 1, env: map[string]string{"FOO": "bar"}},
			},
		},
		{
			name: "license_file inside config is not read as the shorthand",
			content: `test.describe('grp', () => {
  test.use({ teleport: { config: { auth_service: { license_file: 'x.pem' } } } });
});`,
			want: []scopedTeleportConfig{
				{raw: `{ auth_service: { license_file: 'x.pem' } }`, line: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := parseBlocks(strings.Split(tt.content, "\n"))
			got, err := extractTeleportConfigs(tt.content, blocks, "test.spec.ts")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, got, len(tt.want))
			for i := range tt.want {
				require.Equal(t, tt.want[i].raw, got[i].raw)
				require.Equal(t, tt.want[i].license, got[i].license)
				require.Equal(t, tt.want[i].line, got[i].line)
				require.Equal(t, tt.want[i].env, got[i].env)
			}
		})
	}
}

func TestDefaultSelectors(t *testing.T) {
	content := `test.describe('configured', () => {
  test.use({ teleport: { config: { a: 1 } } });
  test('in scope', () => {});
});
test('bare', () => {});
test.describe('other', () => {
  test('nested', () => {});
});`

	configs, err := extractTeleportConfigs(content, parseBlocks(strings.Split(content, "\n")), "x.spec.ts")
	require.NoError(t, err)

	got := defaultSelectors("tests/x.spec.ts", content, configs, 0)
	require.Equal(t, []string{"tests/x.spec.ts:5", "tests/x.spec.ts:7"}, got)

	// A file with no config just runs as a whole
	require.Equal(t, []string{"tests/x.spec.ts"}, defaultSelectors("tests/x.spec.ts", content, nil, 0))
}

func TestNormalizeEnvText(t *testing.T) {
	require.Empty(t, normalizeEnvText(nil))
	require.Empty(t, normalizeEnvText(map[string]string{}))

	// Same content, different key order, normalizes the same
	a := normalizeEnvText(map[string]string{"A": "1", "B": "2"})
	b := normalizeEnvText(map[string]string{"B": "2", "A": "1"})
	require.Equal(t, a, b)

	c := normalizeEnvText(map[string]string{"A": "1", "B": "3"})
	require.NotEqual(t, a, c)
}

func TestNormalizeConfigText(t *testing.T) {
	// Differently-formatted but identical configs normalize the same (so they can be deduped).
	a := normalizeConfigText("{ auth_service: { license_file: 'x.pem' } }")
	b := normalizeConfigText("{\n  auth_service: {\n    license_file: 'x.pem'\n  }\n}")
	require.Equal(t, a, b)

	c := normalizeConfigText("{ auth_service: { license_file: 'y.pem' } }")
	require.NotEqual(t, a, c)
}

func TestScanTeleportConfigsDedupesByConfigAndEnv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.spec.ts", `
test.describe('grp', () => {
  test.use({ teleport: { config: { a: 1 }, env: { FOO: 'bar' } } });
  test('t', () => {});
});
`)
	writeFile(t, dir, "b.spec.ts", `
test.describe('grp', () => {
  test.use({ teleport: { config: { a: 1 }, env: { FOO: 'bar' } } });
  test('t', () => {});
});
`)
	writeFile(t, dir, "c.spec.ts", `
test.describe('grp', () => {
  test.use({ teleport: { config: { a: 1 }, env: { FOO: 'different' } } });
  test('t', () => {});
});
`)

	targets := []scanTarget{
		{path: filepath.Join(dir, "a.spec.ts"), sourceFile: "a.spec.ts"},
		{path: filepath.Join(dir, "b.spec.ts"), sourceFile: "b.spec.ts"},
		{path: filepath.Join(dir, "c.spec.ts"), sourceFile: "c.spec.ts"},
	}

	configs, _, err := scanTeleportConfigs(targets)
	require.NoError(t, err)
	require.Len(t, configs, 2, "same config+env should merge into one batch, different env should stay separate")

	byEnv := make(map[string][]string)
	for _, c := range configs {
		byEnv[c.env["FOO"]] = c.files
	}

	require.ElementsMatch(t, []string{"a.spec.ts:2", "b.spec.ts:2"}, byEnv["bar"])
	require.ElementsMatch(t, []string{"c.spec.ts:2"}, byEnv["different"])
}

func TestScanTeleportConfigsRejectsHelperConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "helper.ts", `
test.use({
  teleport: {
    config: { proxy_service: { ssh_public_addr: ['example.com:3023'] } },
  },
});
`)

	_, _, err := scanTeleportConfigs([]scanTarget{{path: filepath.Join(dir, "helper.ts")}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "helper modules cannot declare teleport:")
}

func TestScanTeleportConfigsAllowsHelperWithoutConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "helper.ts", `test.use({ fixtures: ['connect'] });`)

	_, _, err := scanTeleportConfigs([]scanTarget{{path: filepath.Join(dir, "helper.ts")}})
	require.NoError(t, err)
}

func TestExtractTeleportConfigsRejectsNonInlineOption(t *testing.T) {
	content := `test.describe('d', () => {
  test.use({ teleport: customTeleport });
  test('t', async () => {});
});`
	_, err := extractTeleportConfigs(content, parseBlocks(strings.Split(content, "\n")), "spec.ts")
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be an inline object")
}

func TestScanBrowserRestrictions(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "pinned.spec.ts", `import { test } from '@gravitational/e2e/helpers/test';
test.describe('d', () => {
  test.use({ fixtures: ['ssh-node'], browsers: ['chromium'] });
  test('t', async () => {});
});`)

	writeFile(t, dir, "multi.spec.ts", `import { test } from '@gravitational/e2e/helpers/test';
test.use({ browsers: ["chromium", 'firefox'] });
test('t', async () => {});`)

	writeFile(t, dir, "unpinned.spec.ts", `import { test } from '@gravitational/e2e/helpers/test';
test.use({ fixtures: ['ssh-node'] });
test('t', async () => {});`)

	got, err := scanBrowserRestrictions([]scanTarget{
		{path: filepath.Join(dir, "pinned.spec.ts"), sourceFile: "pinned.spec.ts"},
		{path: filepath.Join(dir, "multi.spec.ts"), sourceFile: "multi.spec.ts"},
		{path: filepath.Join(dir, "unpinned.spec.ts"), sourceFile: "unpinned.spec.ts"},
	})
	require.NoError(t, err)

	require.Equal(t, map[string][]string{
		"pinned.spec.ts": {"chromium"},
		"multi.spec.ts":  {"chromium", "firefox"},
	}, got)
}

func TestScanBrowserRestrictionsIgnoresHelpers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "helper.ts", `test.use({ browsers: ['chromium'] });`)

	// A helper has no spec to attach the restriction to, so it must not silently pin anything.
	got, err := scanBrowserRestrictions([]scanTarget{{path: filepath.Join(dir, "helper.ts")}})
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestScanBrowserRestrictionsRejectsUnknownBrowser(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "typo.spec.ts", `import { test } from '@gravitational/e2e/helpers/test';
test.use({ browsers: ['chrome'] });
test('t', async () => {});`)

	_, err := scanBrowserRestrictions([]scanTarget{
		{path: filepath.Join(dir, "typo.spec.ts"), sourceFile: "typo.spec.ts"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), `invalid browser "chrome"`)
}

func TestScanBrowserRestrictionsRejectsConnectSpec(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shell.spec.ts", `import { test } from '@gravitational/e2e/helpers/test';
test.use({ browsers: ['chromium'] });
test('t', async () => {});`)

	// A Connect spec is routed to the Electron instance, so any browser pin leaves it running nowhere.
	_, err := scanBrowserRestrictions([]scanTarget{
		{path: filepath.Join(dir, "shell.spec.ts"), sourceFile: "tests/connect/shell.spec.ts"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot restrict browsers")
}

func TestExpandedTestFiles(t *testing.T) {
	got := expandedTestFiles([]scanTarget{
		{path: "/repo/e2e/tests/web/authenticated/ssh.spec.ts", sourceFile: "tests/web/authenticated/ssh.spec.ts"},
		{path: "/repo/e2e/tests/web/authenticated/rbac.spec.ts", sourceFile: "tests/web/authenticated/rbac.spec.ts", line: 24},
		// Helper modules are not selectors.
		{path: "/repo/e2e/helpers/test.ts"},
	})

	require.Equal(t, []string{
		"tests/web/authenticated/ssh.spec.ts",
		"tests/web/authenticated/rbac.spec.ts:24",
	}, got)
}

func TestExpandedTestFilesFromResolvedArgs(t *testing.T) {
	e2eDir := t.TempDir()
	specDir := filepath.Join(e2eDir, "tests", "web")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(e2eDir, "tests", "empty"), 0o755))

	for _, name := range []string{"ssh.spec.ts", "rbac.spec.ts"} {
		writeFile(t, specDir, name, `import { test } from '@gravitational/e2e/helpers/test';
test('t', async () => {});`)
	}

	tests := []struct {
		name    string
		args    []string
		want    []string
		wantErr bool
	}{
		{
			name: "file",
			args: []string{"tests/web/ssh.spec.ts"},
			want: []string{"tests/web/ssh.spec.ts"},
		},
		{
			name: "file with line",
			args: []string{"tests/web/ssh.spec.ts:42"},
			want: []string{"tests/web/ssh.spec.ts:42"},
		},
		{
			name: "directory expands to its specs",
			args: []string{"tests/web"},
			want: []string{"tests/web/rbac.spec.ts", "tests/web/ssh.spec.ts"},
		},
		{
			name: "substring filter expands to matches",
			args: []string{"ssh"},
			want: []string{"tests/web/ssh.spec.ts"},
		},
		// A directory holding no specs resolves to nothing, and the caller must not treat that as
		// "run everything".
		{
			name: "directory with no specs",
			args: []string{"tests/empty"},
			want: nil,
		},
		{
			name:    "substring matching nothing",
			args:    []string{"nosuchspec"},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			targets, err := resolveTargetsWithHelpers(suiteDirs{shared: e2eDir, suite: e2eDir}, test.args)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.Equal(t, test.want, expandedTestFiles(targets))
		})
	}
}

func TestFilesForProjectHonoursBrowserRestrictions(t *testing.T) {
	p := &playwrightRunner{config: &e2eConfig{
		browserRestrictions: map[string][]string{
			"tests/web/authenticated/pinned.spec.ts": {"chromium"},
		},
	}}

	files := []string{
		"tests/web/authenticated/pinned.spec.ts:24",
		"tests/web/authenticated/other.spec.ts",
		"tests/connect/shell.spec.ts",
	}

	require.Equal(t, []string{
		"tests/web/authenticated/pinned.spec.ts:24",
		"tests/web/authenticated/other.spec.ts",
	}, p.filesForProject(&testInstance{browser: "chromium"}, files))

	// The pinned spec drops out for other browsers, so its config never triggers a re-init there.
	require.Equal(t, []string{
		"tests/web/authenticated/other.spec.ts",
	}, p.filesForProject(&testInstance{browser: "firefox"}, files))

	// Connect specs are still routed by project, not by the restriction map.
	require.Equal(t, []string{
		"tests/connect/shell.spec.ts",
	}, p.filesForProject(&testInstance{browser: "connect"}, files))
}

func TestNewSuiteDirs(t *testing.T) {
	repoRoot := filepath.Join("/repo")
	shared := filepath.Join(repoRoot, "e2e")

	t.Run("community run reads one tree", func(t *testing.T) {
		dirs := newSuiteDirs(repoRoot, false)

		require.Equal(t, shared, dirs.shared)
		require.Equal(t, shared, dirs.suite)
		require.Equal(t, filepath.Join(shared, "helpers", "test.ts"), dirs.helperPath("e2e/test"))
	})

	t.Run("enterprise run reads its own tree", func(t *testing.T) {
		dirs := newSuiteDirs(repoRoot, true)
		ent := filepath.Join(repoRoot, "e", "e2e")

		require.Equal(t, shared, dirs.shared)
		require.Equal(t, ent, dirs.suite)
		require.Equal(t, filepath.Join(shared, "helpers", "login.ts"), dirs.helperPath("e2e/login"))
		require.Equal(t, filepath.Join(ent, "helpers", "test.ts"), dirs.helperPath("e-e2e/test"))
	})
}

func TestMergeTeleportConfigLicense(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.yaml")
	require.NoError(t, os.WriteFile(base, []byte("auth_service:\n  cluster_name: e2e\n  license_file: base.pem\n"), 0644))

	t.Run("license overrides the base license_file", func(t *testing.T) {
		out := filepath.Join(dir, "license-only.yaml")
		require.NoError(t, mergeTeleportConfig(base, out, dir, "", "/licenses/x.pem"))

		merged := readMergedConfig(t, out)
		auth := merged["auth_service"].(map[string]any)
		require.Equal(t, "/licenses/x.pem", auth["license_file"])
		require.Equal(t, "e2e", auth["cluster_name"])
	})

	t.Run("license merges alongside a declared config", func(t *testing.T) {
		out := filepath.Join(dir, "with-config.yaml")
		require.NoError(t, mergeTeleportConfig(base, out, dir, "{ proxy_service: { enabled: true } }", "/licenses/x.pem"))

		merged := readMergedConfig(t, out)
		require.Equal(t, "/licenses/x.pem", merged["auth_service"].(map[string]any)["license_file"])
		require.Equal(t, true, merged["proxy_service"].(map[string]any)["enabled"])
	})

	t.Run("no license leaves the base untouched", func(t *testing.T) {
		out := filepath.Join(dir, "no-license.yaml")
		require.NoError(t, mergeTeleportConfig(base, out, dir, "", ""))

		merged := readMergedConfig(t, out)
		require.Equal(t, "base.pem", merged["auth_service"].(map[string]any)["license_file"])
	})
}

func readMergedConfig(t *testing.T, path string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var merged map[string]any
	require.NoError(t, yaml.Unmarshal(data, &merged))

	return merged
}

func TestApplyEnterpriseDefaults(t *testing.T) {
	root := filepath.Join("/repo")
	ossBin := filepath.Join(root, "build", "teleport")
	entBin := filepath.Join(root, "e", "build", "teleport")
	entLicense := filepath.Join(root, "e", "fixtures", "license-all-features.pem")

	t.Run("community run is untouched", func(t *testing.T) {
		f := &e2eFlags{teleportBin: ossBin}
		f.applyEnterpriseDefaults(root)

		require.Equal(t, ossBin, f.teleportBin)
		require.Empty(t, f.licenseFile)
	})

	t.Run("enterprise run switches the binary and license", func(t *testing.T) {
		f := &e2eFlags{enterprise: true, teleportBin: ossBin}
		f.applyEnterpriseDefaults(root)

		require.Equal(t, entBin, f.teleportBin)
		require.Equal(t, entLicense, f.licenseFile)
	})

	t.Run("explicit overrides win", func(t *testing.T) {
		f := &e2eFlags{enterprise: true, teleportBin: "/custom/teleport", licenseFile: "/custom.pem"}
		f.applyEnterpriseDefaults(root)

		require.Equal(t, "/custom/teleport", f.teleportBin)
		require.Equal(t, "/custom.pem", f.licenseFile)
	})
}

func TestNormalizeTestFiles(t *testing.T) {
	repoRoot := t.TempDir()
	e2eDir := filepath.Join(repoRoot, "e2e")
	spec := "tests/web/authenticated/e/cloudPanel.spec.ts"

	// run.sh sets E2E_CALLER_DIR to the shell's cwd so paths can be tab-completed from anywhere in
	// the repo, not just from e2e/.
	tests := []struct {
		name      string
		callerDir string
		arg       string
	}{
		{name: "from repo root", callerDir: repoRoot, arg: filepath.Join("e2e", spec)},
		{name: "from e2e dir", callerDir: e2eDir, arg: spec},
		{name: "from a nested dir", callerDir: filepath.Join(e2eDir, "tests"), arg: filepath.Join("..", spec)},
		{name: "absolute", callerDir: repoRoot, arg: filepath.Join(e2eDir, spec)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("E2E_CALLER_DIR", tt.callerDir)

			got, err := normalizeTestFiles(e2eDir, []string{tt.arg})
			require.NoError(t, err)
			require.Equal(t, []string{spec}, got)
		})
	}

	t.Run("playwright line suffix survives normalization", func(t *testing.T) {
		t.Setenv("E2E_CALLER_DIR", repoRoot)

		got, err := normalizeTestFiles(e2eDir, []string{filepath.Join("e2e", spec) + ":42"})
		require.NoError(t, err)
		require.Equal(t, []string{spec + ":42"}, got)
	})
}

func TestFindStringValueAtDepth(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "plain value", body: `{ license: 'featurehiding' }`, want: "featurehiding"},
		{name: "double quotes", body: `{ license: "cloud-ent" }`, want: "cloud-ent"},
		{name: "escaped quote in value", body: `{ license: 'cloud\'ent' }`, want: "cloud'ent"},
		{name: "nested key of the same name is ignored", body: `{ config: { license: 'nested' } }`, want: ""},
		{name: "license_file is not the license key", body: `{ config: { auth_service: { license_file: 'x.pem' } } }`, want: ""},
		{name: "absent", body: `{ config: { a: 1 } }`, want: ""},
		{name: "unterminated", body: `{ license: 'oops`, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, findStringValueAtDepth(tt.body, "license", 1))
		})
	}
}

func TestInferEnterpriseFromArgs(t *testing.T) {
	repoRoot := t.TempDir()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "no paths", want: false},
		{name: "oss spec", args: []string{"e2e/tests/web/authenticated/rbac.spec.ts"}, want: false},
		{name: "enterprise spec", args: []string{"e/e2e/tests/web/authenticated/cloudPanel.spec.ts"}, want: true},
		{name: "enterprise directory", args: []string{"e/e2e/tests"}, want: true},
		{name: "line suffix", args: []string{"e/e2e/tests/web/authenticated/cloudPanel.spec.ts:42"}, want: true},
		{name: "mixed", args: []string{"e2e/tests/web/authenticated/rbac.spec.ts", "e/e2e/tests"}, want: true},
		// e2e/ is not inside e/e2e/, so the shared tree never reads as enterprise.
		{name: "shared tree", args: []string{"e2e/helpers/test.ts"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("E2E_CALLER_DIR", repoRoot)

			f := &e2eFlags{}
			require.NoError(t, f.inferEnterpriseFromArgs(repoRoot, tt.args))
			require.Equal(t, tt.want, f.enterprise)
		})
	}
}
