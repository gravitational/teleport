/*
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

// Unit tests for kqlsafety.go: the KQL injection safety layer.
//
// White-box tests of sanitizeQueryVMsParams, sanitizeKQLValues, quoteKQL, and
// escapeKQL live here. Integration tests that exercise the same code through
// the QueryVMs entry point live in resourcegraph_test.go alongside the rest
// of the QueryVMs behavior coverage.

package azure

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestSanitizeQueryVMsParams is a white-box test of the package's KQL safety
// entry point. Integration coverage through QueryVMs lives in
// resourcegraph_test.go::TestQueryVMsValidatesInputs.
func TestSanitizeQueryVMsParams(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		params      QueryVMsParams
		wantErr     bool
		errContains string // only consulted when wantErr is true
	}{
		// Valid inputs: sanitize succeeds and the returned sanitizedParams
		// echoes every input slice. Equality checks below pin the no-mutation
		// contract; slice-cloning is exercised separately by
		// TestSanitizeQueryVMsParamsCopiesInputSlices.
		{
			name:   "minimal subscription only",
			params: QueryVMsParams{SubscriptionIDs: []string{"11111111-1111-1111-1111-111111111111"}},
		},
		{
			name: "resource group with underscore",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"11111111-1111-1111-1111-111111111111"},
				ResourceGroups:  []string{"my_resource_group"},
			},
		},
		{
			name: "resource group full char-class",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"11111111-1111-1111-1111-111111111111"},
				ResourceGroups:  []string{"My-RG_v1.0(test)"},
			},
		},
		{
			name: "region canonical and with digits",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"11111111-1111-1111-1111-111111111111"},
				Regions:         []string{"eastus", "eastus2"},
			},
		},
		{
			name: "OS type canonical values",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"11111111-1111-1111-1111-111111111111"},
				OSTypes:         []OSType{OSTypeLinux, OSTypeWindows},
			},
		},
		{
			name: "all wildcards",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"11111111-1111-1111-1111-111111111111"},
				Regions:         []string{types.Wildcard},
				ResourceGroups:  []string{types.Wildcard},
				OSTypes:         []OSType{types.Wildcard},
			},
		},
		{
			name: "uppercase UUID subscription ID accepted",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA"},
			},
		},
		{
			name: "lowercase UUID subscription ID accepted",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
			},
		},
		{
			name: "single-character resource group name",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"11111111-1111-1111-1111-111111111111"},
				ResourceGroups:  []string{"a"},
			},
		},
		{
			// Pins that the validator's per-entry checks treat wildcard
			// independently and accept mixed shape. The query meaning of
			// "wildcard absorbs concrete" (resulting predicate omits the
			// whole `| where` clause) is enforced by predicate generation
			// and covered in TestBuildVMDiscoveryKQL; this case covers
			// only the validation surface.
			name: "wildcard mixed with concrete values across every field",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"11111111-1111-1111-1111-111111111111"},
				Regions:         []string{types.Wildcard, "eastus"},
				ResourceGroups:  []string{"rg", types.Wildcard},
				OSTypes:         []OSType{OSTypeLinux, types.Wildcard},
			},
		},

		// Invalid inputs: sanitize returns BadParameter; errContains pins
		// the actionable substring so a regression that collapses the
		// per-branch diagnostics surfaces here.
		{
			name:        "empty subscription list",
			params:      QueryVMsParams{},
			wantErr:     true,
			errContains: "at least one subscription ID is required",
		},
		{
			name:        "empty subscription ID",
			params:      QueryVMsParams{SubscriptionIDs: []string{""}},
			wantErr:     true,
			errContains: "subscription ID must not be empty",
		},
		{
			name:        "untrimmed subscription ID",
			params:      QueryVMsParams{SubscriptionIDs: []string{" sub "}},
			wantErr:     true,
			errContains: "must not have leading or trailing whitespace",
		},
		{
			name: "region display form with space",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
				Regions:         []string{"east us"},
			},
			wantErr:     true,
			errContains: "region",
		},
		{
			name: "resource group with single quote",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
				ResourceGroups:  []string{"rg'name"},
			},
			wantErr:     true,
			errContains: "resource group",
		},
		{
			name: "OS type with digit",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
				OSTypes:         []OSType{"Linux1"},
			},
			wantErr:     true,
			errContains: `OS type "Linux1" must be "Linux" or "Windows"`,
		},
		{
			name: "OS type unknown letter-only value rejected",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
				OSTypes:         []OSType{"Solaris"},
			},
			wantErr:     true,
			errContains: `OS type "Solaris" must be "Linux" or "Windows"`,
		},
		{
			name: "OS type lowercase rejected (strict canonical case)",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
				OSTypes:         []OSType{"linux"},
			},
			wantErr:     true,
			errContains: `OS type "linux" must be "Linux" or "Windows"`,
		},
		{
			name: "OS type uppercase rejected (strict canonical case)",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
				OSTypes:         []OSType{"WINDOWS"},
			},
			wantErr:     true,
			errContains: `OS type "WINDOWS" must be "Linux" or "Windows"`,
		},
		{
			name:        "non-UUID subscription ID rejected",
			params:      QueryVMsParams{SubscriptionIDs: []string{"my-subscription"}},
			wantErr:     true,
			errContains: "must be a canonical UUID",
		},
		{
			name:        "short UUID (missing hex digit) rejected",
			params:      QueryVMsParams{SubscriptionIDs: []string{"00000000-0000-0000-0000-00000000000"}},
			wantErr:     true,
			errContains: "must be a canonical UUID",
		},
		{
			name:        "UUID with wrong separators rejected",
			params:      QueryVMsParams{SubscriptionIDs: []string{"00000000_0000_0000_0000_000000000000"}},
			wantErr:     true,
			errContains: "must be a canonical UUID",
		},
		{
			name: "resource group ending with period rejected",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
				ResourceGroups:  []string{"rg-name."},
			},
			wantErr:     true,
			errContains: "resource group",
		},
		{
			name: "empty region rejected",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
				Regions:         []string{""},
			},
			wantErr:     true,
			errContains: "region must not be empty",
		},
		{
			name: "untrimmed region rejected",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
				Regions:         []string{" eastus"},
			},
			wantErr:     true,
			errContains: "must not have leading or trailing whitespace",
		},
		{
			name: "empty resource group rejected",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
				ResourceGroups:  []string{""},
			},
			wantErr:     true,
			errContains: "resource group must not be empty",
		},
		{
			name: "untrimmed resource group rejected",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
				ResourceGroups:  []string{" rg"},
			},
			wantErr:     true,
			errContains: "must not have leading or trailing whitespace",
		},
		{
			name: "empty OS type rejected",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
				OSTypes:         []OSType{""},
			},
			wantErr:     true,
			errContains: "OS type must not be empty",
		},
		{
			name: "untrimmed OS type rejected",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
				OSTypes:         []OSType{" Linux"},
			},
			wantErr:     true,
			errContains: `OS type " Linux" must not have leading or trailing whitespace`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sanitizeQueryVMsParams(tc.params)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, trace.IsBadParameter(err),
					"validation must surface as BadParameter, got %T: %v", err, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.params.SubscriptionIDs, got.SubscriptionIDs)
			assert.Equal(t, tc.params.Regions, got.Regions)
			assert.Equal(t, tc.params.ResourceGroups, got.ResourceGroups)
			assert.Equal(t, tc.params.OSTypes, got.OSTypes)
		})
	}
}

// TestSanitizeKQLValues is a direct white-box test of the per-field allowlist
// validator. TestSanitizeQueryVMsParams exercises it through the entry point;
// this test pins its main branches (wildcard pass-through, empty, untrimmed,
// regex rejection) in isolation so a future change to either layer surfaces here.
func TestSanitizeKQLValues(t *testing.T) {
	t.Parallel()
	pattern := regexp.MustCompile(`^[a-z]+$`)

	t.Run("valid plus wildcard passes", func(t *testing.T) {
		require.NoError(t, sanitizeKQLValues([]string{"ok", types.Wildcard}, pattern, "test"))
	})

	t.Run("empty rejected", func(t *testing.T) {
		err := sanitizeKQLValues([]string{""}, pattern, "test")
		require.Error(t, err)
		assert.True(t, trace.IsBadParameter(err))
		assert.Contains(t, err.Error(), "must not be empty")
	})

	t.Run("untrimmed rejected", func(t *testing.T) {
		err := sanitizeKQLValues([]string{" ok"}, pattern, "test")
		require.Error(t, err)
		assert.True(t, trace.IsBadParameter(err))
		assert.Contains(t, err.Error(), "leading or trailing whitespace")
	})

	t.Run("regex mismatch rejected", func(t *testing.T) {
		err := sanitizeKQLValues([]string{"bad!"}, pattern, "test")
		require.Error(t, err)
		assert.True(t, trace.IsBadParameter(err))
		assert.Contains(t, err.Error(), "invalid characters")
	})
}

// TestSanitizeQueryVMsParamsCopiesInputSlices pins the "validated once, safe
// thereafter" invariant: caller mutations to the input slices after sanitize
// returns must not leak into the sanitized value. Without slice cloning, a
// caller could pass a benign slice, get a sanitizedParams back, then mutate
// the original slice and have the mutation reach buildVMDiscoveryKQL.
func TestSanitizeQueryVMsParamsCopiesInputSlices(t *testing.T) {
	t.Parallel()
	params := QueryVMsParams{
		SubscriptionIDs: []string{"00000000-0000-0000-0000-000000000000"},
		Regions:         []string{"eastus"},
		ResourceGroups:  []string{"rg"},
		OSTypes:         []OSType{OSTypeLinux},
	}

	sanitized, err := sanitizeQueryVMsParams(params)
	require.NoError(t, err)

	// Mutate every input slice after sanitize returns.
	params.SubscriptionIDs[0] = "11111111-1111-1111-1111-111111111111"
	params.Regions[0] = "' or 1 == 1 or '"
	params.ResourceGroups[0] = "' or 1 == 1 or '"
	params.OSTypes[0] = "' or 1 == 1 or '"

	assert.Equal(t, []string{"00000000-0000-0000-0000-000000000000"}, sanitized.SubscriptionIDs,
		"sanitizedParams must hold a copy of SubscriptionIDs, not share the caller's backing array")
	assert.Equal(t, []string{"eastus"}, sanitized.Regions,
		"sanitizedParams must hold a copy of Regions; otherwise post-validation mutation could inject KQL")
	assert.Equal(t, []string{"rg"}, sanitized.ResourceGroups,
		"sanitizedParams must hold a copy of ResourceGroups; otherwise post-validation mutation could inject KQL")
	assert.Equal(t, []OSType{OSTypeLinux}, sanitized.OSTypes,
		"sanitizedParams must hold a copy of OSTypes; otherwise post-validation mutation could inject KQL")
}

// TestQuoteKQL pins the production single-quoted KQL literal form. FuzzQuoteKQL
// covers round-trip correctness across arbitrary inputs; this test pins
// specific outputs for the most common shapes.
func TestQuoteKQL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", `''`},
		{"plain", "plain", `'plain'`},
		{"single quote doubled", "it's", `'it''s'`},
		{"leading and trailing quote", "'both'", `'''both'''`},
		{"all quotes tripled", "'''", `''''''''`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, quoteKQL(tc.in))
		})
	}
}

func TestEscapeKQL(t *testing.T) {
	t.Parallel()
	assert.Empty(t, escapeKQL(""))
	assert.Equal(t, "plain", escapeKQL("plain"))
	assert.Equal(t, "it''s", escapeKQL("it's"))
	assert.Equal(t, "''both''", escapeKQL("'both'"))
}

// TestDecodeKQLSingleQuoted exercises the quoteKQL inverse directly so a bug in
// the test decoder cannot silently make FuzzEscapeKQL pass for the wrong reasons.
func TestDecodeKQLSingleQuoted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", `''`, ""},
		{"plain", `'plain'`, "plain"},
		{"doubled quote", `'with''quote'`, "with'quote"},
		{"two doubled quotes", `'two''''quotes'`, "two''quotes"},
		{"leading quote", `'''leading'`, "'leading"},
		{"trailing quote", `'trailing'''`, "trailing'"},
		{"backslash literal", `'back\slash'`, `back\slash`},
		{"newline literal", "'line\nbreak'", "line\nbreak"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeKQLSingleQuoted(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	bad := []struct {
		name string
		in   string
	}{
		{"unwrapped", `plain`},
		{"missing closing quote", `'plain`},
		{"missing opening quote", `plain'`},
		{"isolated quote in body", `'east'us'`},
		{"single byte", `'`},
	}
	for _, tt := range bad {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			_, err := decodeKQLSingleQuoted(tt.in)
			require.Error(t, err)
		})
	}
}

// TestDecodeKQLDoubleQuoted exercises the double-quoted KQL decoder directly so
// a decoder bug cannot mask reference-encoder disagreement in FuzzQuoteKQL.
func TestDecodeKQLDoubleQuoted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", `""`, ""},
		{"plain", `"plain"`, "plain"},
		{"escaped double quote", `"with\"quote"`, `with"quote`},
		{"escaped single quote", `"with\'quote"`, `with'quote`},
		{"escaped backslash", `"with\\slash"`, `with\slash`},
		{"null", `"with\0null"`, "with\x00null"},
		{"control chars", `"a\bb\fc\nd\re\tf\vg"`, "a\bb\fc\nd\re\tf\vg"},
		{"unicode bmp", `"é"`, "é"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeKQLDoubleQuoted(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	bad := []struct {
		name string
		in   string
	}{
		{"unwrapped", `plain`},
		{"missing closing quote", `"plain`},
		{"unknown escape", `"\q"`},
		{"truncated unicode", `"\u12"`},
		{"non-hex unicode", `"\uzzzz"`},
		{"single byte", `"`},
	}
	for _, tt := range bad {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			_, err := decodeKQLDoubleQuoted(tt.in)
			require.Error(t, err)
		})
	}
}

// FuzzEscapeKQL is the second half of the C73 defense-in-depth: even if a
// dangerous value ever bypasses sanitizeKQLValues, escapeKQL must produce
// output that cannot break out of the surrounding single-quoted KQL literal.
// Two invariants:
//  1. Round-trip: undoing the quote-doubling recovers the original input.
//  2. No isolated single quote remains in the output.
func FuzzEscapeKQL(f *testing.F) {
	seeds := []string{
		"",
		"plain",
		"with'quote",
		"two''quotes",
		"'leading",
		"trailing'",
		`back\slash`,
		"new\nline",
		"null\x00byte",
		"tab\there",
		"unicode-é",
		"emoji-🎉",
		"' OR 1=1 --",
		"'; drop table; --",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		escaped := escapeKQL(s)
		quoted := quoteKQL(s)

		// Invariant 1: round-trip via the production helpers recovers the
		// input. Using quoteKQL (which wraps escapeKQL) makes the relationship
		// between the two production helpers explicit, and uses the same
		// decoder FuzzQuoteKQL uses so the two fuzzers share a decoding model.
		recovered, err := decodeKQLSingleQuoted(quoted)
		if err != nil {
			t.Fatalf("quoteKQL output cannot be decoded: input %q quoted %q err %v",
				s, quoted, err)
		}
		if recovered != s {
			t.Fatalf("quoteKQL round-trip failed: input %q -> quoted %q -> recovered %q",
				s, quoted, recovered)
		}

		// Invariant 2: every single quote in the output is part of a
		// doubled pair. An isolated quote would close the surrounding
		// KQL literal and let the rest of the input be parsed as KQL.
		for i := 0; i < len(escaped); i++ {
			if escaped[i] != '\'' {
				continue
			}
			if i+1 >= len(escaped) || escaped[i+1] != '\'' {
				t.Fatalf("escapeKQL left an isolated single quote at index %d in %q (input %q)",
					i, escaped, s)
			}
			i++
		}
	})
}

// FuzzQuoteKQL checks quoteKQL against an independent local double-quoted KQL
// reference. The two encoders produce different forms by design: production
// uses single-quoted literals and encodes each interior single quote as two
// consecutive U+0027 bytes; the reference uses double-quoted literals with
// backslash escapes.
//
// String equality between the two outputs is meaningless; we compare by
// decoding both back to the original input and asserting they match.
func FuzzQuoteKQL(f *testing.F) {
	// Seeds intentionally omit astral-plane runes (e.g., "emoji-🎉"): the
	// reference encoder's \uNNNN form skips runes above 0xFFFF, so they would
	// always Skip in the fuzz body. Coverage for emoji etc. lives in
	// FuzzEscapeKQL where the encoder is byte-preserving.
	seeds := []string{
		"",
		"plain",
		"with'quote",
		"two''quotes",
		`back\slash`,
		`mixed'and\both`,
		"new\nline",
		"null\x00byte",
		"tab\there",
		"unicode-é",
		"' OR 1=1 --",
		"'; drop table; --",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		// The dependency-free reference below mirrors Azure Kusto's string quoting
		// behavior, which ranges over runes. Skip inputs where that behavior is
		// intentionally lossy or ambiguous compared to production's byte-preserving
		// single-quote escape.
		if !utf8.ValidString(s) {
			t.Skip()
		}
		for _, r := range s {
			// Azure Kusto's QuoteString emits fmt.Sprintf("\\u%04x", r), which
			// produces 5+ hex digits for astral-plane runes. KQL \u escapes are
			// four hex digits, so skip these rather than make the local decoder
			// accept ambiguous nonstandard output.
			if r > 0xFFFF {
				t.Skip()
			}
		}

		ours := quoteKQL(s)
		theirs := quoteKQLDoubleQuotedReference(s)

		oursDecoded, err := decodeKQLSingleQuoted(ours)
		if err != nil {
			t.Fatalf("quoteKQL produced an undecodable string: input=%q ours=%q err=%v", s, ours, err)
		}
		if oursDecoded != s {
			t.Fatalf("quoteKQL did not faithfully encode input: input=%q ours=%q decoded=%q",
				s, ours, oursDecoded)
		}

		theirsDecoded, err := decodeKQLDoubleQuoted(theirs)
		if err != nil {
			t.Fatalf("double-quoted reference produced an undecodable string: input=%q theirs=%q err=%v",
				s, theirs, err)
		}
		if theirsDecoded != s {
			t.Fatalf("double-quoted reference did not faithfully encode input: input=%q theirs=%q decoded=%q",
				s, theirs, theirsDecoded)
		}

		// Parity: both encoders agreed on the original value.
		if oursDecoded != theirsDecoded {
			t.Fatalf("parity failed: input=%q ours=%q->%q theirs=%q->%q",
				s, ours, oursDecoded, theirs, theirsDecoded)
		}
	})
}

// quoteKQLDoubleQuotedReference is a test-only, dependency-free port of Azure
// Kusto's QuoteString(value, false) string escaping behavior. It intentionally
// uses a different representation than production quoteKQL, which makes
// FuzzQuoteKQL compare semantics rather than output form.
func quoteKQLDoubleQuotedReference(s string) string {
	var out strings.Builder
	out.Grow(len(s) + 2)
	out.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			out.WriteString(`\\`)
		case '\'':
			out.WriteString(`\'`)
		case '"':
			out.WriteString(`\"`)
		case '\x00':
			out.WriteString(`\0`)
		case '\a':
			out.WriteString(`\a`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		case '\v':
			out.WriteString(`\v`)
		default:
			if !shouldEscapeKQLRune(r) {
				out.WriteRune(r)
			} else {
				fmt.Fprintf(&out, `\u%04x`, r)
			}
		}
	}
	out.WriteByte('"')
	return out.String()
}

func shouldEscapeKQLRune(r rune) bool {
	if r <= unicode.MaxLatin1 {
		return unicode.IsControl(r)
	}
	return true
}

// decodeKQLSingleQuoted strips the surrounding single quotes and undoes the
// doubled-interior-single-quote encoding. Errors if the input is not a
// well-formed single-quoted KQL string literal.
//
// Byte-preserving by design, matching production escapeKQL. Future readers:
// do not "fix" this to range over runes; that would silently change the test
// semantics and let escape-related regressions slip through.
func decodeKQLSingleQuoted(s string) (string, error) {
	if len(s) < 2 || s[0] != '\'' || s[len(s)-1] != '\'' {
		return "", fmt.Errorf("not a single-quoted literal: %q", s)
	}
	inner := s[1 : len(s)-1]
	// Walk the body to ensure every interior single quote is doubled.
	var out strings.Builder
	for i := 0; i < len(inner); i++ {
		if inner[i] != '\'' {
			out.WriteByte(inner[i])
			continue
		}
		if i+1 >= len(inner) || inner[i+1] != '\'' {
			return "", fmt.Errorf("isolated single quote at index %d in %q", i, inner)
		}
		out.WriteByte('\'')
		i++
	}
	return out.String(), nil
}

// decodeKQLDoubleQuoted is the inverse of quoteKQLDoubleQuotedReference: strip
// the surrounding double quotes and undo backslash escapes. Mirrors the escape
// set in quoteKQLDoubleQuotedReference plus \uNNNN so decoder tests can catch
// malformed unicode escapes.
func decodeKQLDoubleQuoted(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", fmt.Errorf("not a double-quoted literal: %q", s)
	}
	inner := s[1 : len(s)-1]
	var out strings.Builder
	for i := 0; i < len(inner); i++ {
		if inner[i] != '\\' {
			out.WriteByte(inner[i])
			continue
		}
		if i+1 >= len(inner) {
			return "", fmt.Errorf("trailing backslash in %q", inner)
		}
		switch inner[i+1] {
		case '\\':
			out.WriteByte('\\')
		case '\'':
			out.WriteByte('\'')
		case '"':
			out.WriteByte('"')
		case '0':
			out.WriteByte('\x00')
		case 'a':
			out.WriteByte('\a')
		case 'b':
			out.WriteByte('\b')
		case 'f':
			out.WriteByte('\f')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'v':
			out.WriteByte('\v')
		case 'u':
			if i+5 >= len(inner) {
				return "", fmt.Errorf("truncated \\uNNNN at index %d in %q", i, inner)
			}
			var r rune
			for j := 2; j < 6; j++ {
				c := inner[i+j]
				var d rune
				switch {
				case '0' <= c && c <= '9':
					d = rune(c - '0')
				case 'a' <= c && c <= 'f':
					d = rune(c-'a') + 10
				case 'A' <= c && c <= 'F':
					d = rune(c-'A') + 10
				default:
					return "", fmt.Errorf("non-hex digit %q in \\uNNNN at index %d in %q", c, i, inner)
				}
				r = r*16 + d
			}
			out.WriteRune(r)
			i += 4
		default:
			return "", fmt.Errorf("unknown escape \\%c at index %d in %q", inner[i+1], i, inner)
		}
		i++
	}
	return out.String(), nil
}
