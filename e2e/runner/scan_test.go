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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tmpFile := filepath.Join(dir, "test.spec.ts")
			writeFile(t, dir, "test.spec.ts", tt.content)

			got := scanFile(tmpFile)

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
