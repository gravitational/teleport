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
	"os"
	"path/filepath"
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
		got := scanFixtures(e2eDir, []string{rel})

		if len(got) != 1 {
			t.Fatalf("expected 1 fixture, got %d", len(got))
		}

		if got[0].Name != "connect" {
			t.Errorf("expected fixture name 'connect', got %q", got[0].Name)
		}
	})

	t.Run("web test does not detect Connect fixture", func(t *testing.T) {
		rel, _ := filepath.Rel(e2eDir, filepath.Join(webDir, "roles.spec.ts"))
		got := scanFixtures(e2eDir, []string{rel})

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
