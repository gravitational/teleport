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

// roleFileRe extracts the role filename from a file role reference.
var roleFileRe = regexp.MustCompile(`\bfile:\s*['"]@gravitational/e2e/roles/([^'"]+)['"]`)

// The "key" regexes below all require a word boundary before the key so that
// identifiers like `super_user:` or `myRoles:` don't match as `user:` / `roles:`.

// usersBlockRe matches the beginning of a users array declaration.
var usersBlockRe = regexp.MustCompile(`\busers:\s*\[`)

// userObjRe matches the beginning of a singular user object declaration.
var userObjRe = regexp.MustCompile(`\buser:\s*\{`)

// rolesBlockRe matches the beginning of a roles array declaration.
var rolesBlockRe = regexp.MustCompile(`\broles:\s*\[`)

// traitsBlockRe matches the beginning of a traits object declaration.
var traitsBlockRe = regexp.MustCompile(`\btraits:\s*\{`)

// traitKeyArrayRe matches a trait key followed by an array value (e.g. logins: ['root']).
var traitKeyArrayRe = regexp.MustCompile(`\b(\w+):\s*\[`)

// recordingsBlockRe matches the beginning of a recordings array declaration.
var recordingsBlockRe = regexp.MustCompile(`\brecordings:\s*\[`)

// loginAsBoolRe matches loginAs: true within a user object.
var loginAsBoolRe = regexp.MustCompile(`\bloginAs:\s*true\b`)

// defaultRoleNames returns the built-in roles assigned to users when no roles
// are specified. A fresh slice is returned each call so callers can safely
// mutate or sort the result without affecting other users.
func defaultRoleNames() []scannedRole {
	return []scannedRole{
		{name: "access"},
		{name: "editor"},
	}
}

const testUseCallPrefix = "test.use("

// scannedUser represents a user declaration found in test source code.
// Names are not specified by the test author; they are generated at bootstrap time.
type scannedUser struct {
	roles      []scannedRole
	traits     map[string][]string
	recordings []string
	loginAs    bool
}

// scannedRole represents a role reference found in a user declaration.
// Exactly one of name or file is set.
type scannedRole struct {
	// name is a built-in role like "access", "editor".
	name string
	// file is a role definition file relative to e2e/testdata/roles/, e.g. "viewer.yaml".
	file string
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

// resolveTargetsWithHelpers resolves test file targets and appends any helper
// modules they import, so both fixtures and users declared in helpers are discovered.
func resolveTargetsWithHelpers(e2eDir string, testFiles []string) ([]scanTarget, error) {
	targets, err := resolveFilesToScan(e2eDir, testFiles)
	if err != nil {
		return nil, err
	}

	importedHelpers := make(map[string]bool)
	for _, t := range targets {
		for _, helper := range parseHelperImports(t.path) {
			importedHelpers[helper] = true
		}
	}

	for helper := range importedHelpers {
		targets = append(targets, scanTarget{
			path: filepath.Join(e2eDir, "helpers", helper+".ts"),
		})
	}

	return targets, nil
}

// scanFixturesFromTargets scans pre-resolved targets to discover which fixtures are needed.
func scanFixturesFromTargets(targets []scanTarget) []*fixtures.Fixture {
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

// scanUsersFromTargets scans pre-resolved targets for user declarations.
// Each declaration becomes an independent user; names are generated at bootstrap time.
// When no users are declared, a default user with access and editor roles is returned.
func scanUsersFromTargets(targets []scanTarget) ([]scannedUser, error) {
	var result []scannedUser
	for _, t := range targets {
		users, err := scanFileUsers(t.path, t.line)
		if err != nil {
			return nil, err
		}
		result = append(result, users...)
	}

	if len(result) == 0 {
		return defaultUsers(), nil
	}

	return result, nil
}

// defaultUsers returns a default user with access and editor roles.
func defaultUsers() []scannedUser {
	return []scannedUser{
		{
			roles:   defaultRoleNames(),
			loginAs: true,
		},
	}
}

// scanFileUsers extracts user declarations from test.use() calls in a source file.
// It supports both singular user: { ... } and array users: [{ ... }] forms.
// user, users, and recordings are mutually exclusive within a single test.use() call.
// At most one user in a `users: [...]` array may have `loginAs: true`.
func scanFileUsers(path string, targetLine int) ([]scannedUser, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// Missing files are not fatal: the scanner is used against a
		// heuristic target list that may include paths that don't exist.
		slog.Warn("scan: could not read file", "path", path, "error", err)
		return nil, nil
	}

	lines := strings.Split(string(data), "\n")
	cleaned := stripComments(lines)
	blocks := parseBlocks(cleaned)
	content := strings.Join(cleaned, "\n")

	var result []scannedUser
	for _, call := range findTestUseCalls(content) {
		callLine := 1 + strings.Count(content[:call.start], "\n")

		if targetLine > 0 && !fixtureInScope(callLine, targetLine, blocks) {
			continue
		}

		body := content[call.start:call.end]

		// Detect which top-level options are present in this test.use() call.
		hasUser := userObjRe.MatchString(body)
		hasUsers := usersBlockRe.MatchString(body)
		hasRecordings := hasTopLevelRecordings(body, hasUser, hasUsers)

		// Validate mutual exclusivity.
		var present []string
		if hasUser {
			present = append(present, "user")
		}
		if hasUsers {
			present = append(present, "users")
		}
		if hasRecordings {
			present = append(present, "recordings")
		}

		if len(present) > 1 {
			return nil, fmt.Errorf(
				"%s:%d: user, users, and recordings are mutually exclusive in test.use() (found: %s)",
				path, callLine, strings.Join(present, ", "),
			)
		}

		var users []scannedUser

		if hasUser {
			if loc := userObjRe.FindStringIndex(body); loc != nil {
				userBlock := extractInner(body[loc[0]:], '{', '}')
				if userBlock != "" {
					user := parseUserBlock(userBlock)
					user.loginAs = true // singular user is implicitly loginAs
					users = append(users, user)
				}
			}
		} else if hasUsers {
			if loc := usersBlockRe.FindStringIndex(body); loc != nil {
				usersContent := extractInner(body[loc[0]:], '[', ']')
				if usersContent != "" {
					for _, userBlock := range extractAllOuter(usersContent, '{', '}') {
						users = append(users, parseUserBlock(userBlock))
					}
				}
			}

			loginAsCount := 0
			for _, u := range users {
				if u.loginAs {
					loginAsCount++
				}
			}
			if loginAsCount > 1 {
				return nil, fmt.Errorf(
					"%s:%d: at most one user in users: [...] may have loginAs: true (found %d)",
					path, callLine, loginAsCount,
				)
			}
		} else if hasRecordings {
			topRecordings := scanTopLevelRecordings(body)
			if len(topRecordings) > 0 {
				users = append(users, scannedUser{
					roles:      defaultRoleNames(),
					recordings: topRecordings,
					loginAs:    true,
				})
			}
		}

		result = append(result, users...)
	}

	return result, nil
}

// hasTopLevelRecordings returns true if the test.use() body contains a recordings: [...]
// that is NOT inside a user: {} or users: [] block.
func hasTopLevelRecordings(body string, hasUser, hasUsers bool) bool {
	loc := recordingsBlockRe.FindStringIndex(body)
	if loc == nil {
		return false
	}

	// If there's no user/users block, any recordings: [...] is top-level.
	if !hasUser && !hasUsers {
		return true
	}

	// Check if the recordings match is inside a user: {} block.
	if hasUser {
		if userLoc := userObjRe.FindStringIndex(body); userLoc != nil {
			userEnd := findClosingDelim(body, userLoc[0], '{', '}')
			if userEnd > 0 && loc[0] > userLoc[0] && loc[0] < userEnd {
				return false
			}
		}
	}

	// Check if the recordings match is inside a users: [] block.
	if hasUsers {
		if usersLoc := usersBlockRe.FindStringIndex(body); usersLoc != nil {
			usersEnd := findClosingDelim(body, usersLoc[0], '[', ']')
			if usersEnd > 0 && loc[0] > usersLoc[0] && loc[0] < usersEnd {
				return false
			}
		}
	}

	return true
}

// findClosingDelim finds the position after the matching close delimiter
// for the first open delimiter at or after pos. Returns -1 if not found.
// Delimiter bytes inside string or template literals are ignored.
func findClosingDelim(s string, pos int, open, close byte) int {
	start := strings.IndexByte(s[pos:], open)
	if start < 0 {
		return -1
	}

	depth := 0
	var quote byte

	for i := pos + start; i < len(s); i++ {
		ch := s[i]

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
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}

	return -1
}

// scanTopLevelRecordings extracts recordings: [...] from a test.use() body.
// This is only called when no user/users block is present (they are mutually exclusive).
func scanTopLevelRecordings(body string) []string {
	loc := recordingsBlockRe.FindStringIndex(body)
	if loc == nil {
		return nil
	}

	content := extractInner(body[loc[0]:], '[', ']')
	if content == "" {
		return nil
	}

	var recordings []string
	for _, m := range fixtureRefRe.FindAllStringSubmatch(content, -1) {
		recordings = append(recordings, m[1])
	}

	return recordings
}

// parseUserBlock extracts roles, traits, and loginAs from a single user object block.
func parseUserBlock(userBlock string) scannedUser {
	var user scannedUser

	rolesLoc := rolesBlockRe.FindStringIndex(userBlock)
	if rolesLoc != nil {
		rolesContent := extractInner(userBlock[rolesLoc[0]:], '[', ']')
		if rolesContent != "" {
			for _, m := range roleFileRe.FindAllStringSubmatch(rolesContent, -1) {
				user.roles = append(user.roles, scannedRole{file: m[1]})
			}

			withoutFileRoles := roleFileRe.ReplaceAllString(rolesContent, "")
			for _, m := range fixtureRefRe.FindAllStringSubmatch(withoutFileRoles, -1) {
				user.roles = append(user.roles, scannedRole{name: m[1]})
			}
		}
	}

	traitsLoc := traitsBlockRe.FindStringIndex(userBlock)
	if traitsLoc != nil {
		traitsContent := extractInner(userBlock[traitsLoc[0]:], '{', '}')
		if traitsContent != "" {
			user.traits = parseTraits(traitsContent)
		}
	}

	recLoc := recordingsBlockRe.FindStringIndex(userBlock)
	if recLoc != nil {
		recContent := extractInner(userBlock[recLoc[0]:], '[', ']')
		if recContent != "" {
			for _, m := range fixtureRefRe.FindAllStringSubmatch(recContent, -1) {
				user.recordings = append(user.recordings, m[1])
			}
		}
	}

	if loginAsBoolRe.MatchString(userBlock) {
		user.loginAs = true
	}

	sortRoles(user.roles)

	return user
}

// extractInner returns the content between the first open delimiter and its matching close.
// Delimiter bytes inside string or template literals are ignored.
func extractInner(s string, open, close byte) string {
	start := strings.IndexByte(s, open)
	if start < 0 {
		return ""
	}

	depth := 0
	var quote byte

	for i := start; i < len(s); i++ {
		ch := s[i]

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
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[start+1 : i]
			}
		}
	}

	return ""
}

// parseTraits parses trait key-value pairs from the content of a traits object block.
// e.g. `logins: ['root', 'alice'], groups: ['dev']` → {"logins": ["root", "alice"], "groups": ["dev"]}
func parseTraits(traitsContent string) map[string][]string {
	traits := make(map[string][]string)

	for _, m := range traitKeyArrayRe.FindAllStringSubmatchIndex(traitsContent, -1) {
		key := traitsContent[m[2]:m[3]]
		bracketContent := extractInner(traitsContent[m[0]:], '[', ']')
		if bracketContent == "" {
			continue
		}

		for _, ref := range fixtureRefRe.FindAllStringSubmatch(bracketContent, -1) {
			traits[key] = append(traits[key], ref[1])
		}
	}

	return traits
}

// extractAllOuter returns each top-level open...close block (including delimiters) from s.
// Delimiter bytes inside string or template literals are ignored.
func extractAllOuter(s string, open, close byte) []string {
	var blocks []string
	depth := 0
	start := -1
	var quote byte

	for i := 0; i < len(s); i++ {
		ch := s[i]

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
		case open:
			if depth == 0 {
				start = i
			}
			depth++
		case close:
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
func sortRoles(roles []scannedRole) {
	slices.SortStableFunc(roles, func(a, b scannedRole) int {
		// Built-in names (name set) come before file refs (file set).
		aIsName := a.name != ""
		bIsName := b.name != ""

		if aIsName != bIsName {
			if aIsName {
				return -1
			}

			return 1
		}

		if aIsName {
			return strings.Compare(a.name, b.name)
		}

		return strings.Compare(a.file, b.file)
	})
}

