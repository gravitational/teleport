/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package yamlmod

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// isPlainText reports whether s is valid UTF-8 containing no control characters,
// matching the realistic input domain for config paths and values.
func isPlainText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	return !strings.ContainsFunc(s, unicode.IsControl)
}

// FuzzModify exercises the path-based mutation operations against arbitrary
// YAML inputs and paths. The package does raw *yaml.Node tree surgery, so the
// goal is to ensure it never panics and never produces a document that fails to
// round-trip back through the parser.
func FuzzModify(f *testing.F) {
	seeds := []string{
		"teleport:\n  data_dir: /var/lib/teleport\n",
		"a:\n  b:\n    c: 1\n",
		"list:\n- one\n- two\n",
		"# comment\nteleport:\n  join_params:\n    method: token\n    token_name: abc\n",
		"top: scalar\n",
		"nested:\n  arr:\n  - name: x\n    uri: y\n",
		"",
	}
	paths := []string{
		"teleport.data_dir",
		"teleport.join_params.method",
		"a.b.c",
		"new.path.here",
		"list",
		"nested.arr[0].name",
		"a[0]",
		"",
	}
	for _, s := range seeds {
		for _, p := range paths {
			f.Add([]byte(s), p, "fuzzval")
		}
	}

	f.Fuzz(func(t *testing.T, data []byte, path, value string) {
		// Restrict path/value to control-char-free valid UTF-8, matching the real
		// input domain (CLI flags and --set values: addresses, tokens, paths). This
		// keeps the fuzzer focused on the structural tree surgery and avoids known
		// yaml.v3 encoder quirks (e.g. a value with an embedded newline+tab is
		// emitted as a block scalar that yaml.v3 itself cannot re-parse), which
		// cannot arise from real inputs and are not this package's concern.
		if !isPlainText(path) || !isPlainText(value) {
			return
		}

		doc, err := Parse(data)
		if err != nil {
			return // invalid YAML or non-document input: nothing to mutate.
		}

		// A successfully parsed, unmodified document must always render to YAML
		// that parses again.
		mustRoundTrip(t, doc)

		// Read operations must never panic on arbitrary paths, regardless of
		// whether the path resolves.
		_, _ = Get(doc, path)
		_ = Exists(doc, path)

		if err := Set(doc, path, value); err == nil {
			mustRoundTrip(t, doc)
			// A scalar Set with no array index must be observable via Get:
			// if Set claims success, the value has to actually be there.
			if !strings.Contains(path, "[") {
				got, err := Get(doc, path)
				if err != nil {
					t.Fatalf("Get after successful Set(%q) failed: %v", path, err)
				}
				if got != value {
					t.Fatalf("Get after Set(%q)=%q returned %q", path, value, got)
				}
			}
		}

		if err := SetBool(doc, path, true); err == nil {
			mustRoundTrip(t, doc)
		}

		if err := Delete(doc, path); err == nil {
			mustRoundTrip(t, doc)
			// A successful non-indexed Delete must remove the path.
			if !strings.Contains(path, "[") && Exists(doc, path) {
				t.Fatalf("path %q still exists after successful Delete", path)
			}
		}
	})
}

func mustRoundTrip(t *testing.T, doc *yaml.Node) {
	t.Helper()
	out, err := Render(doc)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if _, err := Parse(out); err != nil {
		t.Fatalf("rendered output did not round-trip: %v\noutput:\n%s", err, out)
	}
}
