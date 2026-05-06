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

var (
	fixtureArrayRe     = regexp.MustCompile(`fixtures:\s*\[+([^]]*)]`)
	lineNumberSuffixRe = regexp.MustCompile(`:\d+$`)
	fixtureRefRe       = regexp.MustCompile(`['"]([^'"]+)['"]`)
	helperImportRe     = regexp.MustCompile(`from\s+['"]@gravitational/e2e/helpers/(\w+)['"]`)
	roleFileRe         = regexp.MustCompile(`\bfile:\s*['"]@gravitational/e2e/roles/([^'"]+)['"]`)

	// The "key" regexes below require a word boundary so identifiers like
	// `super_user:` or `myRoles:` don't match as `user:` / `roles:`.
	usersBlockRe     = regexp.MustCompile(`\busers:\s*\[`)
	userObjRe        = regexp.MustCompile(`\buser:\s*\{`)
	rolesBlockRe     = regexp.MustCompile(`\broles:\s*\[`)
	traitsBlockRe    = regexp.MustCompile(`\btraits:\s*\{`)
	traitKeyArrayRe  = regexp.MustCompile(`\b(\w+):\s*\[`)
	loginAsBoolRe    = regexp.MustCompile(`\bloginAs:\s*true\b`)
)

// defaultRoleNames returns the built-in roles assigned when no roles are specified. Returns a fresh slice each call so callers can mutate/sort safely.
func defaultRoleNames() []scannedRole {
	return []scannedRole{
		{name: "access"},
		{name: "editor"},
	}
}

const testUseCallPrefix = "test.use("

// scannedUser is a user declaration discovered in test source. Names are generated at bootstrap time, not by the test author.
type scannedUser struct {
	roles   []scannedRole
	traits  map[string][]string
	loginAs bool
	// arrayIdx is the position within a `users: [...]` array; nil otherwise. Keeps duplicate-by-content entries addressable as distinct accounts via loginAs(N).
	arrayIdx *int
}

// scannedRole is a role reference; exactly one of name (built-in like "access") or file (e.g. "viewer.yaml" under e2e/testdata/roles/) is set.
type scannedRole struct {
	name string
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

// resolveTargetsWithHelpers resolves test files plus any helper modules they import, so fixtures and users declared in helpers are also discovered.
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

// scanFixtures wraps resolveTargetsWithHelpers + scanFixturesFromTargets for callers that haven't been split yet.
func scanFixtures(e2eDir string, testFiles []string) []*fixtures.Fixture {
	targets, err := resolveTargetsWithHelpers(e2eDir, testFiles)
	if err != nil {
		slog.Warn("fixture scan: error resolving files", "error", err)

		return nil
	}

	return scanFixturesFromTargets(targets)
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

// findInlineComment returns the byte offset of the first // not inside a string/template literal, or -1.
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

// scanUsersFromTargets scans pre-resolved targets for user declarations and always appends the default access/editor user so implicit-auth specs (no test.use(), :authenticated project) resolve a username in mixed runs.
func scanUsersFromTargets(targets []scanTarget) ([]scannedUser, error) {
	var result []scannedUser
	for _, t := range targets {
		users, err := scanFileUsers(t.path, t.line)
		if err != nil {
			return nil, err
		}
		result = append(result, users...)
	}

	return ensureDefaultUser(result)
}

// ensureDefaultUser appends the default user unless an explicit declaration already produces the same canonical key.
func ensureDefaultUser(users []scannedUser) ([]scannedUser, error) {
	keys := make(map[string]bool, len(users))
	for _, u := range users {
		k, err := canonicalUserKey(u)
		if err != nil {
			return nil, err
		}
		keys[k] = true
	}

	for _, du := range defaultUsers() {
		k, err := canonicalUserKey(du)
		if err != nil {
			return nil, err
		}
		if keys[k] {
			continue
		}
		users = append(users, du)
	}

	return users, nil
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

// scanFileUsers extracts user declarations from test.use() calls. Singular `user: {}` and array `users: [...]` are mutually exclusive per call; at most one array entry may have `loginAs: true`.
func scanFileUsers(path string, targetLine int) ([]scannedUser, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// Helper module paths are guessed from import names (see
		// resolveTargetsWithHelpers), so the file may not exist. Warn
		// but continue rather than failing the whole scan.
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

		hasUser := userObjRe.MatchString(body)
		hasUsers := usersBlockRe.MatchString(body)

		if hasUser && hasUsers {
			return nil, fmt.Errorf(
				"%s:%d: user and users are mutually exclusive in test.use()",
				path, callLine,
			)
		}

		var users []scannedUser

		if hasUser {
			if loc := userObjRe.FindStringIndex(body); loc != nil {
				userBlock := extractInner(body[loc[0]:], '{', '}')
				if userBlock != "" {
					user := parseUserBlock(userBlock)
					user.loginAs = true // singular user is implicitly loginAs
					warnDuplicateRoles(path, callLine, user.roles)
					users = append(users, user)
				}
			}
		} else if hasUsers {
			if loc := usersBlockRe.FindStringIndex(body); loc != nil {
				usersContent := extractInner(body[loc[0]:], '[', ']')
				if usersContent != "" {
					for i, userBlock := range extractAllOuter(usersContent, '{', '}') {
						u := parseUserBlock(userBlock)
						idx := i
						u.arrayIdx = &idx
						warnDuplicateRoles(path, callLine, u.roles)
						users = append(users, u)
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
		}

		result = append(result, users...)
	}

	return result, nil
}

// scanBalanced returns the index of the close delimiter matching the open at openIdx, or -1 if unmatched. Delimiters inside string/template literals are ignored.
func scanBalanced(s string, openIdx int, open, close byte) int {
	depth := 0
	var quote byte

	for i := openIdx; i < len(s); i++ {
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
				return i
			}
		}
	}

	return -1
}

// parseUserBlock extracts roles, traits, and loginAs from a single user object block.
func parseUserBlock(userBlock string) scannedUser {
	var user scannedUser

	rolesLoc := rolesBlockRe.FindStringIndex(userBlock)
	if rolesLoc != nil {
		rolesContent := extractInner(userBlock[rolesLoc[0]:], '[', ']')
		if rolesContent != "" {
			// Collect file-role ranges first, then walk all quoted strings and
			// classify each based on whether it falls inside a file-role match.
			fileMatches := roleFileRe.FindAllStringSubmatchIndex(rolesContent, -1)
			for _, m := range fileMatches {
				user.roles = append(user.roles, scannedRole{file: rolesContent[m[2]:m[3]]})
			}

			fi := 0
			for _, loc := range fixtureRefRe.FindAllStringSubmatchIndex(rolesContent, -1) {
				for fi < len(fileMatches) && fileMatches[fi][1] <= loc[0] {
					fi++
				}
				if fi < len(fileMatches) && loc[0] >= fileMatches[fi][0] && loc[0] < fileMatches[fi][1] {
					continue
				}

				user.roles = append(user.roles, scannedRole{name: rolesContent[loc[2]:loc[3]]})
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

	if loginAsBoolRe.MatchString(userBlock) {
		user.loginAs = true
	}

	sortRoles(user.roles)

	return user
}

// extractInner returns the content between the first open delimiter and its matching close, ignoring delimiters inside string/template literals.
func extractInner(s string, open, close byte) string {
	start := strings.IndexByte(s, open)
	if start < 0 {
		return ""
	}

	end := scanBalanced(s, start, open, close)
	if end < 0 {
		return ""
	}

	return s[start+1 : end]
}

// parseTraits parses trait key-value pairs (e.g. `logins: ['root', 'alice'], groups: ['dev']`) into a map.
func parseTraits(traitsContent string) map[string][]string {
	traits := make(map[string][]string)

	for _, m := range traitKeyArrayRe.FindAllStringSubmatchIndex(traitsContent, -1) {
		key := traitsContent[m[2]:m[3]]
		// traitKeyArrayRe ends at the byte after `[`, so m[1]-1 is the `[` byte.
		bracketOpen := m[1] - 1
		bracketClose := scanBalanced(traitsContent, bracketOpen, '[', ']')
		if bracketClose < 0 {
			continue
		}

		bracketContent := traitsContent[bracketOpen+1 : bracketClose]
		for _, ref := range fixtureRefRe.FindAllStringSubmatch(bracketContent, -1) {
			traits[key] = append(traits[key], ref[1])
		}
	}

	return traits
}

// extractAllOuter returns each top-level open...close block from s (including delimiters), ignoring delimiters inside string/template literals.
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

func warnDuplicateRoles(path string, line int, roles []scannedRole) {
	for i := 1; i < len(roles); i++ {
		if roles[i] != roles[i-1] {
			continue
		}

		ref := roles[i].name
		if roles[i].file != "" {
			ref = "file:" + roles[i].file
		}
		slog.Warn("scan: duplicate role for user", "path", path, "line", line, "role", ref)
	}
}

// sortRoles sorts roles with built-in names before file refs, alphabetical within each group.
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

