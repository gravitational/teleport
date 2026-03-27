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
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gravitational/teleport/e2e/runner/fixtures"
)

// fixtureArrayRe matches fixture arrays inside test.use() calls, including multiline declarations.
//   - test.use({ fixtures: ['ssh-node'] })
//   - test.use({ fixtures: [['connect'], { option: true }] })
var fixtureArrayRe = regexp.MustCompile(`test\.use\([^)]*fixtures:\s*\[+([^]]*)]`)

// fixtureRefRe extracts individual quoted fixture names from the matched array contents.
var fixtureRefRe = regexp.MustCompile(`'([^']+)'`)

// helperImportRe matches imports from the e2e helpers package and captures the module name.
// e.g. `from '@gravitational/e2e/helpers/connect'` → "connect"
var helperImportRe = regexp.MustCompile(`from\s+['"]@gravitational/e2e/helpers/(\w+)['"]`)

// scanFixtures scans test files and the helpers they import to discover which fixtures are needed.
func scanFixtures(e2eDir string, testFiles []string) []*fixtures.Fixture {
	specFiles, err := resolveFilesToScan(e2eDir, testFiles)
	if err != nil {
		slog.Debug("fixture scan: error resolving files", "error", err)

		return nil
	}

	slog.Debug("fixture scan: resolved spec files", "count", len(specFiles), "files", specFiles)

	// Helpers can also reference fixtures (such as Connect), so we need to scan them as well.
	importedHelpers := make(map[string]bool)
	for _, file := range specFiles {
		for _, helper := range parseHelperImports(file) {
			importedHelpers[helper] = true
		}
	}

	filesToScan := specFiles
	for helper := range importedHelpers {
		helperPath := filepath.Join(e2eDir, "helpers", helper+".ts")

		if _, err := os.Stat(helperPath); err == nil {
			filesToScan = append(filesToScan, helperPath)
		}
	}

	slog.Debug("fixture scan: total files to scan", "count", len(filesToScan))

	seen := make(map[string]struct{})
	var result []*fixtures.Fixture

	for _, file := range filesToScan {
		for _, f := range scanFile(file) {
			if _, ok := seen[f.Name]; ok {
				continue
			}

			seen[f.Name] = struct{}{}
			result = append(result, f)
		}
	}

	return result
}

func resolveFilesToScan(e2eDir string, testFiles []string) ([]string, error) {
	if len(testFiles) == 0 {
		return walkSpecFiles(filepath.Join(e2eDir, "tests"))
	}

	var files []string
	for _, tf := range testFiles {
		abs := filepath.Join(e2eDir, tf)

		info, err := os.Stat(abs)
		if err != nil {
			continue
		}

		if info.IsDir() {
			matches, err := walkSpecFiles(abs)
			if err != nil {
				return nil, err
			}

			files = append(files, matches...)

			continue
		}

		files = append(files, abs)
	}

	return files, nil
}

func walkSpecFiles(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".spec.ts") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

func parseHelperImports(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var helpers []string
	for _, match := range helperImportRe.FindAllSubmatch(data, -1) {
		helpers = append(helpers, string(match[1]))
	}

	return helpers
}

func scanFile(path string) []*fixtures.Fixture {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	// Strip single-line comment lines before matching so that commented-out fixture declarations are not detected.
	var filtered []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}

		filtered = append(filtered, line)
	}

	// Match against the joined content so that fixture arrays spanning multiple lines are detected correctly.
	content := strings.Join(filtered, "\n")

	var result []*fixtures.Fixture
	for _, m := range fixtureArrayRe.FindAllStringSubmatch(content, -1) {
		for _, ref := range fixtureRefRe.FindAllStringSubmatch(m[1], -1) {
			if f := fixtures.FindByName(ref[1]); f != nil {
				result = append(result, f)
			}
		}
	}

	return result
}
