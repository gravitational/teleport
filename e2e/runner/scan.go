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
	traitKeyArrayRe = regexp.MustCompile(`(?:'([^'\\]+)'|"([^"\\]+)"|\b(\w+))\s*:\s*\[`)
	loginAsBoolRe   = regexp.MustCompile(`\bloginAs:\s*true\b`)
)

// defaultRoleNames returns the built-in roles assigned when no roles are
// specified. Returns a fresh slice each call so callers can mutate/sort safely.
func defaultRoleNames() []scannedRole {
	return []scannedRole{
		{name: "access"},
		{name: "editor"},
	}
}

const testUseCallPrefix = "test.use("

// scannedUser is a user declaration discovered in test source. Names are
// generated at bootstrap time, not by the test author.
type scannedUser struct {
	roles      []scannedRole
	traits     map[string][]string
	recordings []string
	loginAs    bool
	// arrayIdx is the position within a `users: [...]` array; nil otherwise.
	// Keeps duplicate-by-content entries addressable as distinct accounts via
	// loginAs(N).
	arrayIdx *int
	// isDefault marks the implicit default user so its canonical key is
	// distinct from any explicit `user: {}` declaration with the same roles.
	isDefault bool
	// sourceFile is the spec path (relative to e2eDir) where this user was
	// declared inline; empty for the implicit default user.
	sourceFile string
}

// scannedRole is a role reference; exactly one of name (built-in like
// "access") or file (e.g. "viewer.yaml" under e2e/testdata/roles/) is set.
type scannedRole struct {
	name string
	file string
}

// scanTarget represents a file to scan with an optional line constraint.
type scanTarget struct {
	path string
	line int // 0 means scan entire file
	// sourceFile, when non-empty, is the spec path emitted in the canonical
	// key for users declared in this target. Empty for helper modules.
	sourceFile string
}

// blockRange represents a brace-delimited block in a source file (1-indexed
// lines; byte offsets index into the joined comment-stripped content).
type blockRange struct {
	start, end         int
	startByte, endByte int
}

// callRange represents the byte offsets of a test.use(...) call in the content string.
type callRange struct {
	start, end int
}

// resolveTargetsWithHelpers resolves test files plus any helper modules they
// import, so fixtures and users declared in helpers are also discovered.
func resolveTargetsWithHelpers(e2eDir string, testFiles []string) ([]scanTarget, error) {
	targets, err := resolveFilesToScan(e2eDir, testFiles)
	if err != nil {
		return nil, err
	}

	for i := range targets {
		if rel, err := filepath.Rel(e2eDir, targets[i].path); err == nil {
			targets[i].sourceFile = rel
		}
	}

	importedHelpers := make(map[string]bool)
	for _, t := range targets {
		for _, helper := range parseHelperImports(t.path) {
			importedHelpers[helper] = true
		}
	}

	helpersBase := cmp.Or(os.Getenv("E2E_SHARED_DIR"), e2eDir)
	for helper := range importedHelpers {
		targets = append(targets, scanTarget{
			path: filepath.Join(helpersBase, "helpers", helper+".ts"),
		})
	}

	return targets, nil
}

// scanFixturesFromTargets scans pre-resolved targets to discover which
// fixtures are needed.
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

// scanFixtures wraps resolveTargetsWithHelpers + scanFixturesFromTargets for
// callers that haven't been split yet.
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
		if targetLine > 0 && !fixtureInScope(call.start, targetLine, blocks) {
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

// findInlineComment returns the byte offset of the first // not inside a
// string/template literal, or -1.
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

// findBlockCommentOpen returns the byte offset of the first /* that is not
// inside a string literal, or -1.
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
	lineStarts := make([]int, len(lines))
	off := 0
	for i, line := range lines {
		lineStarts[i] = off
		off += len(line) + 1
	}

	type stackEntry struct {
		line int
		off  int
	}

	var blocks []blockRange
	var stack []stackEntry
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
				stack = append(stack, stackEntry{line: lineNum, off: lineStarts[i] + j})
			case '}':
				if len(stack) > 0 {
					top := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					blocks = append(blocks, blockRange{
						start:     top.line,
						end:       lineNum,
						startByte: top.off,
						endByte:   lineStarts[i] + j,
					})
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

func fixtureInScope(callStart, targetLine int, blocks []blockRange) bool {
	enclosing := smallestEnclosingBlock(callStart, blocks)
	if enclosing == nil {
		return true
	}

	return targetLine >= enclosing.start && targetLine <= enclosing.end
}

// scanUsersFromTargets scans pre-resolved targets for user declarations and
// always appends the default access/editor user so implicit-auth specs resolve
// a username in mixed runs. The default user has a distinct canonical key
// (`"default": true`), so explicit `user: {}` declarations with the same roles
// still get their own account.
func scanUsersFromTargets(targets []scanTarget) ([]scannedUser, error) {
	var result []scannedUser
	for _, t := range targets {
		users, err := scanFileUsers(t.path, t.line, t.sourceFile)
		if err != nil {
			return nil, err
		}
		result = append(result, users...)
	}

	return append(result, defaultUsers()...), nil
}

// defaultUsers returns a default user with access and editor roles.
func defaultUsers() []scannedUser {
	return []scannedUser{
		{
			roles:     defaultRoleNames(),
			loginAs:   true,
			isDefault: true,
		},
	}
}

// scanFileUsers extracts user declarations from test.use() calls. Singular
// `user: {}`, array `users: [...]`, and `recordings` are mutually exclusive
// per call; at most one array entry may have `loginAs: true`. Helper modules
// are rejected if they declare user/users/recordings since runtime would
// merge them into every importing spec.
func scanFileUsers(path string, targetLine int, sourceFile string) ([]scannedUser, error) {
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

	type useCall struct {
		line  int
		start int
	}

	var (
		result            []scannedUser
		recordingsCalls   []useCall
		explicitUserCalls []useCall
	)
	for _, call := range findTestUseCalls(content) {
		callLine := 1 + strings.Count(content[:call.start], "\n")

		if targetLine > 0 && !fixtureInScope(call.start, targetLine, blocks) {
			continue
		}

		body := content[call.start:call.end]

		userOpen, userClose := findKeyValueAtDepth(body, "user", '{', '}', 1)
		usersOpen, usersClose := findKeyValueAtDepth(body, "users", '[', ']', 1)
		hasUser := userOpen >= 0
		hasUsers := usersOpen >= 0
		hasRecordings := hasTopLevelRecordings(body)

		uc := useCall{line: callLine, start: call.start}
		if hasUser || hasUsers {
			explicitUserCalls = append(explicitUserCalls, uc)
		}
		if hasRecordings {
			recordingsCalls = append(recordingsCalls, uc)
		}

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
			userBlock := body[userOpen+1 : userClose-1]
			if userBlock != "" {
				user := parseUserBlock(userBlock)
				user.loginAs = true // singular user is implicitly loginAs
				warnDuplicateRoles(path, callLine, user.roles)
				users = append(users, user)
			}
		} else if hasUsers {
			usersContent := body[usersOpen+1 : usersClose-1]
			if usersContent != "" {
				for i, raw := range extractAllOuter(usersContent, '{', '}') {
					u := parseUserBlock(raw[1 : len(raw)-1])
					idx := i
					u.arrayIdx = &idx
					warnDuplicateRoles(path, callLine, u.roles)
					users = append(users, u)
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
					isDefault:  true,
				})
			}
		}

		for i := range users {
			if users[i].isDefault {
				continue
			}
			users[i].sourceFile = sourceFile
		}
		result = append(result, users...)
	}

	for _, rec := range recordingsCalls {
		for _, user := range explicitUserCalls {
			if scopesOverlap(rec.start, user.start, blocks) {
				return nil, fmt.Errorf(
					"%s:%d: top-level recordings: cannot coexist with user:/users: at line %d in an overlapping scope; "+
						"combine them into a single test.use({ user: { ..., recordings: [...] } })",
					path, rec.line, user.line,
				)
			}
		}
	}

	if sourceFile == "" && (len(recordingsCalls) > 0 || len(explicitUserCalls) > 0) {
		var line int
		if len(explicitUserCalls) > 0 {
			line = explicitUserCalls[0].line
		} else {
			line = recordingsCalls[0].line
		}
		return nil, fmt.Errorf(
			"%s:%d: helper modules cannot declare user:/users:/top-level recordings: in test.use(); declare them inline in the spec",
			path, line,
		)
	}

	return result, nil
}

// scopesOverlap reports whether two test.use() calls apply to any common
// test. Two scopes overlap when at least one is file-level (no enclosing block)
// or one's enclosing block contains the other's.
func scopesOverlap(call1Start, call2Start int, blocks []blockRange) bool {
	b1 := smallestEnclosingBlock(call1Start, blocks)
	b2 := smallestEnclosingBlock(call2Start, blocks)
	if b1 == nil || b2 == nil {
		return true
	}
	return blockContains(*b1, *b2) || blockContains(*b2, *b1)
}

func smallestEnclosingBlock(callStart int, blocks []blockRange) *blockRange {
	var best *blockRange
	for i := range blocks {
		b := &blocks[i]
		if b.startByte < callStart && b.endByte > callStart {
			if best == nil || (b.endByte-b.startByte) < (best.endByte-best.startByte) {
				best = b
			}
		}
	}
	return best
}

func blockContains(outer, inner blockRange) bool {
	return outer.startByte <= inner.startByte && outer.endByte >= inner.endByte
}

// hasTopLevelRecordings reports whether the test.use({...}) body has a
// recordings: [...] key directly on the args object (depth 1), ignoring any
// recordings: nested inside a value object/array. Depth-aware so nested keys
// in user/users/traits/etc. don't false-positive.
func hasTopLevelRecordings(body string) bool {
	open, _ := findTopLevelRecordingsArray(body)
	return open >= 0
}

// findTopLevelRecordingsArray returns the byte offsets [open, close) of the
// `[...]` value of a `recordings:` key at depth 1 of the test.use args object.
// Returns (-1, -1) if no such key exists.
func findTopLevelRecordingsArray(body string) (int, int) {
	return findKeyValueAtDepth(body, "recordings", '[', ']', 1)
}

func findKeyValueAtDepth(body, key string, openBracket, closeBracket byte, targetDepth int) (int, int) {
	depth := 0
	var quote byte

	for i := 0; i < len(body); i++ {
		ch := body[i]

		if quote != 0 {
			if ch == '\\' {
				i++
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}

		switch ch {
		case '\'', '"', '`':
			quote = ch
			continue
		}

		if depth == targetDepth {
			if bracketAt, ok := matchesKeyAt(body, i, key, openBracket); ok {
				closeIdx := scanBalanced(body, bracketAt, openBracket, closeBracket)
				if closeIdx < 0 {
					return -1, -1
				}
				return bracketAt, closeIdx + 1
			}
		}

		switch ch {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		}
	}

	return -1, -1
}

func matchesKeyAt(body string, i int, key string, bracket byte) (int, bool) {
	if i+len(key) > len(body) {
		return 0, false
	}
	if body[i:i+len(key)] != key {
		return 0, false
	}
	if i > 0 && isIdentChar(body[i-1]) {
		return 0, false
	}

	j := i + len(key)
	for j < len(body) && isWhitespace(body[j]) {
		j++
	}
	if j >= len(body) || body[j] != ':' {
		return 0, false
	}
	j++
	for j < len(body) && isWhitespace(body[j]) {
		j++
	}
	if j >= len(body) || body[j] != bracket {
		return 0, false
	}
	return j, true
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '_' || b == '$'
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// scanBalanced returns the index of the close delimiter matching the open at
// openIdx, or -1 if unmatched. Delimiters inside string/template literals are
// ignored.
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

// findClosingDelim finds the position after the matching close delimiter
// for the first open delimiter at or after pos. Returns -1 if not found.
// Delimiter bytes inside string or template literals are ignored.
func findClosingDelim(s string, pos int, open, close byte) int {
	start := strings.IndexByte(s[pos:], open)
	if start < 0 {
		return -1
	}

	end := scanBalanced(s, pos+start, open, close)
	if end < 0 {
		return -1
	}

	return end + 1
}

// scanTopLevelRecordings extracts recordings: [...] from a test.use() body,
// only when the recordings: key is at depth 1 of the args object.
func scanTopLevelRecordings(body string) []string {
	open, close := findTopLevelRecordingsArray(body)
	if open < 0 {
		return nil
	}

	inner := body[open+1 : close-1]

	var recordings []string
	for _, m := range fixtureRefRe.FindAllStringSubmatch(inner, -1) {
		recordings = append(recordings, m[1])
	}

	return recordings
}

// parseUserBlock extracts roles, traits, recordings, and loginAs from a single
// user object block. Callers MUST pass content with any surrounding `{}` already stripped.
func parseUserBlock(userBlock string) scannedUser {
	var user scannedUser

	if open, close := findKeyValueAtDepth(userBlock, "roles", '[', ']', 0); open >= 0 {
		rolesContent := userBlock[open+1 : close-1]
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

	if open, close := findKeyValueAtDepth(userBlock, "traits", '{', '}', 0); open >= 0 {
		traitsContent := userBlock[open+1 : close-1]
		if traitsContent != "" {
			user.traits = parseTraits(traitsContent)
		}
	}

	if open, close := findKeyValueAtDepth(userBlock, "recordings", '[', ']', 0); open >= 0 {
		recContent := userBlock[open+1 : close-1]
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

// extractInner returns the content between the first open delimiter and its
// matching close, ignoring delimiters inside string/template literals.
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

// parseTraits parses trait key-value pairs
// (e.g. `logins: ['root', 'alice'], groups: ['dev']`) into a map.
func parseTraits(traitsContent string) map[string][]string {
	traits := make(map[string][]string)

	for _, m := range traitKeyArrayRe.FindAllStringSubmatchIndex(traitsContent, -1) {
		// traitKeyArrayRe has three alternation groups for the key:
		// m[2:3] single-quoted, m[4:5] double-quoted, m[6:7] bare identifier.
		// Exactly one group matches per match; the others have index -1.
		var key string
		switch {
		case m[2] >= 0:
			key = traitsContent[m[2]:m[3]]
		case m[4] >= 0:
			key = traitsContent[m[4]:m[5]]
		default:
			key = traitsContent[m[6]:m[7]]
		}

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

// extractAllOuter returns each top-level open...close block from s (including
// delimiters), ignoring delimiters inside string/template literals.
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

// sortRoles sorts roles with built-in names before file refs, alphabetical
// within each group.
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

