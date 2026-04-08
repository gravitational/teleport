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
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gravitational/teleport/e2e/runner/fixtures"
)

// fixtureArrayRe matches fixture array declarations within a test.use() call body.
//   - fixtures: ['ssh-node']
//   - fixtures: [['connect'], { option: true }]
var fixtureArrayRe = regexp.MustCompile(`fixtures:\s*\[+([^]]*)]`)

// lineNumberSuffixRe matches a trailing :line_number on a test path (e.g. "my-spec.ts:42").
var lineNumberSuffixRe = regexp.MustCompile(`:\d+$`)

// fixtureRefRe extracts individual quoted fixture names from the matched array contents.
var fixtureRefRe = regexp.MustCompile(`['"]([^'"]+)['"]`)

// helperImportRe matches imports from the e2e helpers package and captures the module name.
// e.g. `from '@gravitational/e2e/helpers/connect'` → "connect"
var helperImportRe = regexp.MustCompile(`from\s+['"]@gravitational/e2e/helpers/(\w+)['"]`)

// userNameRe extracts the user name from a user object declaration.
var userNameRe = regexp.MustCompile(`name:\s*['"]([^'"]+)['"]`)

// roleFileRe extracts the role filename from a file role reference.
var roleFileRe = regexp.MustCompile(`file:\s*['"]@gravitational/e2e/roles/([^'"]+)['"]`)

// loginAsRe extracts the loginAs value from a test.use() call.
var loginAsRe = regexp.MustCompile(`loginAs:\s*['"]([^'"]+)['"]`)

// usersBlockRe matches the beginning of a users array declaration.
var usersBlockRe = regexp.MustCompile(`users:\s*\[`)

// rolesBlockRe matches the beginning of a roles array declaration.
var rolesBlockRe = regexp.MustCompile(`roles:\s*\[`)

const testUseCallPrefix = "test.use("

// ScannedUser represents a user declaration found in test source code.
type ScannedUser struct {
	Name  string
	Roles []ScannedRole
}

// ScannedRole represents a role reference found in a user declaration.
// Exactly one of Name or File is set.
type ScannedRole struct {
	// Name is a built-in role like "access", "editor".
	Name string
	// File is a role definition file relative to e2e/testdata/roles/, e.g. "viewer.yaml".
	File string
}

// scanTarget represents a file to scan with an optional line constraint.
type scanTarget struct {
	path string
	line int // 0 means scan entire file
}

// blockRange represents a brace-delimited block in a source file (1-indexed lines).
type blockRange struct {
	start, end int
}

// callRange represents the byte offsets of a test.use(...) call in the content string.
type callRange struct {
	start, end int
}

// scanFixtures scans test files and the helpers they import to discover which fixtures are needed.
func scanFixtures(e2eDir string, testFiles []string) []*fixtures.Fixture {
	targets, err := resolveFilesToScan(e2eDir, testFiles)
	if err != nil {
		slog.Warn("fixture scan: error resolving files", "error", err)

		return nil
	}

	slog.Debug("fixture scan: resolved targets", "count", len(targets))

	// Helpers can also reference fixtures (such as Connect), so we need to scan them as well.
	importedHelpers := make(map[string]bool)
	for _, t := range targets {
		for _, helper := range parseHelperImports(t.path) {
			importedHelpers[helper] = true
		}
	}

	// Helpers are always scanned fully (no line targeting).
	// No existence check needed — scanFile handles missing files gracefully.
	for helper := range importedHelpers {
		targets = append(targets, scanTarget{
			path: filepath.Join(e2eDir, "helpers", helper+".ts"),
		})
	}

	slog.Debug("fixture scan: total files to scan", "count", len(targets))

	seen := make(map[string]struct{})
	var result []*fixtures.Fixture

	for _, t := range targets {
		for _, f := range scanFile(t.path, t.line) {
			if _, ok := seen[f.Name]; ok {
				continue
			}

			seen[f.Name] = struct{}{}
			result = append(result, f)
		}
	}

	return result
}

func resolveFilesToScan(e2eDir string, testFiles []string) ([]scanTarget, error) {
	if len(testFiles) == 0 {
		paths, err := walkSpecFiles(filepath.Join(e2eDir, "tests"))
		if err != nil {
			return nil, err
		}

		targets := make([]scanTarget, len(paths))
		for i, p := range paths {
			targets[i] = scanTarget{path: p}
		}

		return targets, nil
	}

	// Cache the full spec file list lazily for substring filter fallback,
	// so we walk the tree at most once even with multiple filter arguments.
	var allSpecs []string

	var targets []scanTarget
	for _, tf := range testFiles {
		// Extract optional Playwright :line suffix (e.g. "my-spec.ts:42").
		var line int
		if loc := lineNumberSuffixRe.FindStringIndex(tf); loc != nil {
			var err error
			line, err = strconv.Atoi(tf[loc[0]+1:])
			if err != nil {
				return nil, err
			}

			tf = tf[:loc[0]]
		}

		abs := filepath.Join(e2eDir, tf)

		info, err := os.Stat(abs)
		if err == nil {
			if info.IsDir() {
				matches, err := walkSpecFiles(abs)
				if err != nil {
					return nil, err
				}

				for _, m := range matches {
					targets = append(targets, scanTarget{path: m})
				}
			} else {
				targets = append(targets, scanTarget{path: abs, line: line})
			}

			continue
		}

		// Not a concrete path — treat as a Playwright substring filter
		// and match against all spec files.
		if allSpecs == nil {
			allSpecs, err = walkSpecFiles(filepath.Join(e2eDir, "tests"))
			if err != nil {
				return nil, err
			}
		}

		before := len(targets)
		for _, spec := range allSpecs {
			rel, _ := filepath.Rel(e2eDir, spec)
			if strings.Contains(rel, tf) {
				targets = append(targets, scanTarget{path: spec, line: line})
			}
		}

		if len(targets) == before {
			return nil, fmt.Errorf("test path %q did not resolve to any spec files", tf)
		}
	}

	return targets, nil
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

	lines := strings.Split(string(data), "\n")
	cleaned := strings.Join(stripComments(lines), "\n")

	var helpers []string
	for _, match := range helperImportRe.FindAllStringSubmatch(cleaned, -1) {
		helpers = append(helpers, match[1])
	}

	return helpers
}

func scanFile(path string, targetLine int) []*fixtures.Fixture {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	cleaned := stripComments(lines)
	blocks := parseBlocks(cleaned)
	content := strings.Join(cleaned, "\n")

	var result []*fixtures.Fixture
	for _, call := range findTestUseCalls(content) {
		callLine := 1 + strings.Count(content[:call.start], "\n")

		if targetLine > 0 && !fixtureInScope(callLine, targetLine, blocks) {
			continue
		}

		body := content[call.start:call.end]
		for _, m := range fixtureArrayRe.FindAllStringSubmatch(body, -1) {
			for _, ref := range fixtureRefRe.FindAllStringSubmatch(m[1], -1) {
				if f := fixtures.FindByName(ref[1]); f != nil {
					result = append(result, f)
				}
			}
		}
	}

	return result
}

func stripComments(lines []string) []string {
	cleaned := make([]string, len(lines))
	inBlock := false

	for i, line := range lines {
		if inBlock {
			if idx := strings.Index(line, "*/"); idx >= 0 {
				inBlock = false
				line = line[idx+2:]
			} else {
				continue
			}
		}

		if idx := findBlockCommentOpen(line); idx >= 0 {
			if endIdx := strings.Index(line[idx+2:], "*/"); endIdx >= 0 {
				// Single-line block comment.
				line = line[:idx] + line[idx+2+endIdx+2:]
			} else {
				inBlock = true
				line = line[:idx]
			}
		}

		// Strip trailing // comment that is outside string literals.
		if idx := findInlineComment(line); idx >= 0 {
			line = line[:idx]
		}

		cleaned[i] = line
	}

	return cleaned
}

// findInlineComment returns the byte offset of the first // that is not inside a single-quoted, double-quoted, or
// backtick string literal, or -1.
func findInlineComment(line string) int {
	var quote byte

	for i := 0; i < len(line); i++ {
		ch := line[i]

		if quote != 0 {
			if ch == '\\' {
				i++
			} else if ch == quote {
				quote = 0
			}

			continue
		}

		switch ch {
		case '\'', '"', '`':
			quote = ch
		case '/':
			if i+1 < len(line) && line[i+1] == '/' {
				return i
			}
		}
	}

	return -1
}

// findBlockCommentOpen returns the byte offset of the first /* that is not inside a string literal, or -1.
func findBlockCommentOpen(line string) int {
	var quote byte

	for i := 0; i < len(line); i++ {
		ch := line[i]

		if quote != 0 {
			if ch == '\\' {
				i++
			} else if ch == quote {
				quote = 0
			}

			continue
		}

		switch ch {
		case '\'', '"', '`':
			quote = ch
		case '/':
			if i+1 < len(line) && line[i+1] == '*' {
				return i
			}
		}
	}

	return -1
}

func parseBlocks(lines []string) []blockRange {
	var blocks []blockRange
	var stack []int
	inTemplateLiteral := false

	for i, line := range lines {
		lineNum := i + 1
		var quote byte

		for j := 0; j < len(line); j++ {
			ch := line[j]

			if inTemplateLiteral && quote == 0 {
				quote = '`'
			}

			if quote != 0 {
				if ch == '\\' {
					j++
				} else if ch == quote {
					if quote == '`' {
						inTemplateLiteral = false
					}

					quote = 0
				}

				continue
			}

			switch ch {
			case '\'', '"':
				quote = ch
			case '`':
				quote = '`'
				inTemplateLiteral = true
			case '{':
				stack = append(stack, lineNum)
			case '}':
				if len(stack) > 0 {
					start := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					blocks = append(blocks, blockRange{start: start, end: lineNum})
				}
			}
		}

		if quote != 0 && quote != '`' {
			quote = 0
		}
	}

	return blocks
}

func findTestUseCalls(content string) []callRange {
	var calls []callRange
	offset := 0

	for {
		idx := strings.Index(content[offset:], testUseCallPrefix)
		if idx < 0 {
			break
		}

		callStart := offset + idx
		// Start paren counting after the opening '(' in "test.use("
		depth := 1
		pos := callStart + len(testUseCallPrefix)
		var quote byte

		for pos < len(content) && depth > 0 {
			ch := content[pos]

			if quote != 0 {
				if ch == '\\' {
					pos++ // skip escaped character
				} else if ch == quote {
					quote = 0
				}

				pos++

				continue
			}

			switch ch {
			case '\'', '"', '`':
				quote = ch
			case '(':
				depth++
			case ')':
				depth--
			}

			pos++
		}

		if depth == 0 {
			calls = append(calls, callRange{start: callStart, end: pos})
		}

		offset = pos
	}

	return calls
}

func fixtureInScope(fixtureLine, targetLine int, blocks []blockRange) bool {
	var enclosing *blockRange

	for i := range blocks {
		b := &blocks[i]
		if fixtureLine > b.start && fixtureLine < b.end {
			if enclosing == nil || (b.end-b.start) < (enclosing.end-enclosing.start) {
				enclosing = b
			}
		}
	}

	if enclosing == nil {
		return true
	}

	return targetLine >= enclosing.start && targetLine <= enclosing.end
}

// scanUsers scans all test files for user declarations and returns a deduplicated list.
// It always includes the implicit bob user with access and editor roles.
func scanUsers(e2eDir string, testFiles []string) []ScannedUser {
	targets, err := resolveFilesToScan(e2eDir, testFiles)
	if err != nil {
		slog.Warn("user scan: error resolving files", "error", err)

		return defaultUsers()
	}

	byName := make(map[string]*ScannedUser)

	for _, t := range targets {
		for _, u := range scanFileUsers(t.path, t.line) {
			if existing, ok := byName[u.Name]; ok {
				existing.Roles = mergeRoles(existing.Roles, u.Roles)
			} else {
				clone := u
				byName[u.Name] = &clone
			}
		}
	}

	// Always include implicit bob.
	for _, u := range defaultUsers() {
		if existing, ok := byName[u.Name]; ok {
			existing.Roles = mergeRoles(existing.Roles, u.Roles)
		} else {
			clone := u
			byName[u.Name] = &clone
		}
	}

	result := make([]ScannedUser, 0, len(byName))
	for _, u := range byName {
		sortRoles(u.Roles)
		result = append(result, *u)
	}

	slices.SortStableFunc(result, func(a, b ScannedUser) int {
		return strings.Compare(a.Name, b.Name)
	})

	return result
}

// defaultUsers returns the implicit bob user with access and editor roles.
func defaultUsers() []ScannedUser {
	return []ScannedUser{
		{
			Name: "bob",
			Roles: []ScannedRole{
				{Name: "access"},
				{Name: "editor"},
			},
		},
	}
}

// scanFileUsers extracts user declarations from test.use() calls in a source file.
func scanFileUsers(path string, targetLine int) []ScannedUser {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	cleaned := stripComments(lines)
	blocks := parseBlocks(cleaned)
	content := strings.Join(cleaned, "\n")

	var result []ScannedUser
	for _, call := range findTestUseCalls(content) {
		callLine := 1 + strings.Count(content[:call.start], "\n")

		if targetLine > 0 && !fixtureInScope(callLine, targetLine, blocks) {
			continue
		}

		body := content[call.start:call.end]

		loc := usersBlockRe.FindStringIndex(body)
		if loc == nil {
			continue
		}

		// Extract from after "users: " up to end of body, then get the bracket block.
		usersContent := extractBracketBlock(body[loc[0]:])
		if usersContent == "" {
			continue
		}

		for _, userBlock := range extractBraceBlocks(usersContent) {
			nameMatch := userNameRe.FindStringSubmatch(userBlock)
			if nameMatch == nil {
				continue
			}

			user := ScannedUser{Name: nameMatch[1]}

			rolesLoc := rolesBlockRe.FindStringIndex(userBlock)
			if rolesLoc != nil {
				rolesContent := extractBracketBlock(userBlock[rolesLoc[0]:])
				if rolesContent != "" {
					// Extract file roles first.
					for _, m := range roleFileRe.FindAllStringSubmatch(rolesContent, -1) {
						user.Roles = append(user.Roles, ScannedRole{File: m[1]})
					}

					// Remove file role references, then extract remaining quoted strings as built-in role names.
					withoutFileRoles := roleFileRe.ReplaceAllString(rolesContent, "")
					for _, m := range fixtureRefRe.FindAllStringSubmatch(withoutFileRoles, -1) {
						user.Roles = append(user.Roles, ScannedRole{Name: m[1]})
					}
				}
			}

			sortRoles(user.Roles)
			result = append(result, user)
		}
	}

	return result
}

// scanFileLoginAs extracts the loginAs value from test.use() calls in a source file.
func scanFileLoginAs(path string, targetLine int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	cleaned := stripComments(lines)
	blocks := parseBlocks(cleaned)
	content := strings.Join(cleaned, "\n")

	for _, call := range findTestUseCalls(content) {
		callLine := 1 + strings.Count(content[:call.start], "\n")

		if targetLine > 0 && !fixtureInScope(callLine, targetLine, blocks) {
			continue
		}

		body := content[call.start:call.end]
		if m := loginAsRe.FindStringSubmatch(body); m != nil {
			return m[1]
		}
	}

	return ""
}

// extractBracketBlock returns the content between the first '[' and its matching ']'.
func extractBracketBlock(s string) string {
	start := strings.IndexByte(s, '[')
	if start < 0 {
		return ""
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return s[start+1 : i]
			}
		}
	}

	return ""
}

// extractBraceBlocks returns each top-level { ... } block from input.
func extractBraceBlocks(s string) []string {
	var blocks []string
	depth := 0
	start := -1

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				blocks = append(blocks, s[start:i+1])
				start = -1
			}
		}
	}

	return blocks
}

// sortRoles sorts roles so that built-in names come before file refs,
// alphabetical within each group.
func sortRoles(roles []ScannedRole) {
	slices.SortStableFunc(roles, func(a, b ScannedRole) int {
		// Built-in names (Name set) come before file refs (File set).
		aIsName := a.Name != ""
		bIsName := b.Name != ""

		if aIsName != bIsName {
			if aIsName {
				return -1
			}

			return 1
		}

		if aIsName {
			return strings.Compare(a.Name, b.Name)
		}

		return strings.Compare(a.File, b.File)
	})
}

// mergeRoles deduplicates roles by name or file, returning the union.
func mergeRoles(a, b []ScannedRole) []ScannedRole {
	seen := make(map[string]struct{})
	var result []ScannedRole

	for _, r := range a {
		key := "name:" + r.Name
		if r.File != "" {
			key = "file:" + r.File
		}

		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, r)
		}
	}

	for _, r := range b {
		key := "name:" + r.Name
		if r.File != "" {
			key = "file:" + r.File
		}

		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, r)
		}
	}

	return result
}
