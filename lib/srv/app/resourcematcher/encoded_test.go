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

package resourcematcher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEncodedCharsConstructorValidation pins that GlobEncoded and CaptureEncoded
// admit only the separator "/" and reject an empty set or any other char. The
// model permits an encoded char only when it is structurally meaningful and
// byte-faithful to forward, and "/" is the one such char today.
func TestEncodedCharsConstructorValidation(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		wantErr bool
	}{
		{"slash ok", []string{"/"}, false},
		{"duplicate slash ok", []string{"/", "/"}, false},
		{"empty set", nil, true},
		{"empty string", []string{""}, true},
		{"at sign", []string{"@"}, true},
		{"colon", []string{":"}, true},
		{"percent", []string{"%"}, true},
		{"encoded slash literal", []string{"%2F"}, true},
		{"dot", []string{"."}, true},
		{"slash plus other", []string{"/", "@"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gErr := GlobEncoded(tt.allowed)
			_, cErr := CaptureEncoded("x", tt.allowed)
			if tt.wantErr {
				require.Error(t, gErr, "GlobEncoded should reject %v", tt.allowed)
				require.Error(t, cErr, "CaptureEncoded should reject %v", tt.allowed)
				return
			}
			require.NoError(t, gErr, "GlobEncoded should admit %v", tt.allowed)
			require.NoError(t, cErr, "CaptureEncoded should admit %v", tt.allowed)
		})
	}
}

// TestEncodedCharsRejectedAtLoad pins that a set naming any char other than "/"
// fails when the rule compiles, on every encoded node that takes one:
// glob_encoded and capture_encoded. Catching it at load turns a per-request
// evaluation error into a clear compile failure, so a misconfigured rule never
// reaches a request.
func TestEncodedCharsRejectedAtLoad(t *testing.T) {
	bad := []string{
		`path.match(literal("a", glob_encoded(set("@"))))`,
		`path.match(literal("a", capture_encoded("x", set("@"))))`,
		`path.match(literal("a", glob_encoded(set("/", "@"))))`,
		`path.match(literal("a", glob_encoded(set("%2F"))))`,
		`path.match(literal("a", capture_encoded("x", set(":"))))`,
	}
	for _, p := range bad {
		t.Run("bad/"+p, func(t *testing.T) {
			_, err := compileExpression(p)
			require.Error(t, err, "expected a load error for %q", p)
		})
	}

	good := []string{
		`path.match(literal("a", glob_encoded(set("/"))))`,
		`path.match(literal("a", capture_encoded("x", set("/"))))`,
	}
	for _, p := range good {
		t.Run("good/"+p, func(t *testing.T) {
			_, err := compileExpression(p)
			require.NoError(t, err)
		})
	}
}

// TestEncodedCharsEndToEnd exercises the full rule path through CompileRoles and
// RoleSet.Evaluate. An encoded node is the sole opt-in: it matches both plain
// and encoded segments, a plain node rejects an encoded one, and any
// non-separator or double escape is rejected at tokenize before a rule runs.
func TestEncodedCharsEndToEnd(t *testing.T) {
	const encGlob = `path.match(literal("p", glob_encoded(set("/"))))`
	const encCapture = `path.match(literal("p", capture_encoded("id", set("/"))))`
	const plainGlob = `path.match(literal("p", glob()))`

	tests := []struct {
		name        string
		pred        string
		path        string
		wantAllowed bool
		wantDeny    DenyKind
		wantVars    map[string]string
	}{
		{"glob_encoded matches encoded", encGlob, "/p/mygroup%2Fmyproject", true, "", map[string]string{}},
		{"glob_encoded matches plain", encGlob, "/p/plain", true, "", map[string]string{}},
		{"capture_encoded binds decoded upper hex", encCapture, "/p/a%2Fb", true, "", map[string]string{"id": "a/b"}},
		{"capture_encoded binds decoded lower hex", encCapture, "/p/a%2fb", true, "", map[string]string{"id": "a/b"}},
		{"capture_encoded binds plain", encCapture, "/p/plain", true, "", map[string]string{"id": "plain"}},
		{"plain glob rejects encoded", plainGlob, "/p/a%2Fb", false, DenyNotAllowed, nil},
		{"plain glob matches plain", plainGlob, "/p/plain", true, "", map[string]string{}},
		{"other escape rejected at tokenize", encGlob, "/p/a%40b", false, DenyInvalidRequest, nil},
		{"dot escape rejected at tokenize", encGlob, "/p/a%2Eb", false, DenyInvalidRequest, nil},
		{"double-encoded slash rejected at tokenize", encGlob, "/p/a%252Fb", false, DenyInvalidRequest, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, err := CompileRoles([]Role{{Name: "r", Expressions: []string{tt.pred}}})
			require.NoError(t, err)
			got, err := set.Evaluate(Request{Method: "GET", Path: tt.path}, Identity{})
			require.NoError(t, err)
			require.Equal(t, tt.wantAllowed, got.Allowed, tt.path)
			if tt.wantAllowed {
				require.Equal(t, tt.wantVars, got.Allow.Vars)
				return
			}
			require.Equal(t, tt.wantDeny, got.Deny.Kind, tt.path)
		})
	}
}

// TestEncodedLiteralConstructorValidation pins the encoded_literal value rules:
// the value is the decoded form, so each "/"-separated part must be a legal,
// non-empty segment, never relative, and never carry a "%". The allowed set is
// held to the same "/"-only rule as the other encoded nodes.
func TestEncodedLiteralConstructorValidation(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		allowed []string
		wantErr bool
	}{
		{"single segment", "lodash", []string{"/"}, false},
		{"two parts", "mygroup/myproject", []string{"/"}, false},
		{"nested parts", "security/sub/tool", []string{"/"}, false},
		{"tilde is content", "a~b/c", []string{"/"}, false},
		{"at and colon are content", "@scope/sha256:abc", []string{"/"}, false},
		{"empty value", "", []string{"/"}, true},
		{"empty interior part", "a//b", []string{"/"}, true},
		{"trailing empty part", "a/", []string{"/"}, true},
		{"relative dot", "a/./b", []string{"/"}, true},
		{"relative dotdot", "a/../b", []string{"/"}, true},
		{"percent in value", "a%2Fb", []string{"/"}, true},
		{"illegal byte", "a b", []string{"/"}, true},
		{"empty set", "a/b", nil, true},
		{"other char in set", "a/b", []string{"@"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := EncodedLiteral(tt.value, tt.allowed)
			if tt.wantErr {
				require.Error(t, err, "value=%q allowed=%v", tt.value, tt.allowed)
				return
			}
			require.NoError(t, err, "value=%q allowed=%v", tt.value, tt.allowed)
		})
	}
}

// TestEncodedLiteralEndToEnd pins encoded_literal through the full rule path: it
// matches one segment by its decoded value, in either hex case, spanning the
// encoded slashes of a nested group, but only when the slash actually arrives
// encoded. A real slash splits into two segments and does not match a single
// encoded_literal.
func TestEncodedLiteralEndToEnd(t *testing.T) {
	const group = `path.match(literal("p", encoded_literal("mygroup/myproject", set("/"))))`
	const nested = `path.match(literal("p", encoded_literal("security/sub/tool", set("/"))))`
	const plain = `path.match(literal("p", encoded_literal("lodash", set("/"))))`

	tests := []struct {
		name        string
		pred        string
		path        string
		wantAllowed bool
		wantDeny    DenyKind
	}{
		{"matches upper hex", group, "/p/mygroup%2Fmyproject", true, ""},
		{"matches lower hex", group, "/p/mygroup%2fmyproject", true, ""},
		{"matches mixed hex", group, "/p/mygroup%2fmyproject", true, ""},
		{"matches nested group", nested, "/p/security%2Fsub%2Ftool", true, ""},
		{"value mismatch denies", group, "/p/other%2Fthing", false, DenyNotAllowed},
		{"a real slash does not match a single encoded_literal", group, "/p/mygroup/myproject", false, DenyNotAllowed},
		{"plain value matches plain segment", plain, "/p/lodash", true, ""},
		{"double-encoded slash rejected at tokenize", group, "/p/mygroup%252Fmyproject", false, DenyInvalidRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, err := CompileRoles([]Role{{Name: "r", Expressions: []string{tt.pred}}})
			require.NoError(t, err)
			got, err := set.Evaluate(Request{Method: "GET", Path: tt.path}, Identity{})
			require.NoError(t, err)
			require.Equal(t, tt.wantAllowed, got.Allowed, tt.path)
			if !tt.wantAllowed {
				require.Equal(t, tt.wantDeny, got.Deny.Kind, tt.path)
			}
		})
	}
}

// TestByteFidelityNeverReEncode pins the core forwarding guarantee: the matcher
// forwards the raw request bytes and never re-encodes them. A "~", "@", or ":"
// stays literal in a captured value, never turning into "%7E", "%40", or "%3A"
// the way a quote(safe="") re-encode would. The other half of the guarantee is
// that an encoded form of those chars never enters in the first place: the
// strict tokenizer admits only the encoded separator, so "%7E" and friends are
// rejected outright.
func TestByteFidelityNeverReEncode(t *testing.T) {
	// A plain capture binds the raw segment, with every legal pchar punctuation
	// kept exactly as sent.
	plain, err := compileExpression(`path.match(literal("p", capture("x")))`)
	require.NoError(t, err)
	got, err := plain.Evaluate(Request{Method: "GET", Path: "/p/a~b@c:d-e._f~"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "a~b@c:d-e._f~", got.Allow.Vars["x"], "raw bytes forwarded, nothing re-encoded")

	// capture_encoded decodes the separator for the vars view but leaves every
	// other char alone, so the "~", "@", and ":" are never re-encoded and only
	// the "%2F" resolves to "/". The raw token still forwards byte-faithfully
	// upstream; this asserts the decoded vars value the where would compare.
	enc, err := compileExpression(`path.match(literal("p", capture_encoded("x", set("/"))))`)
	require.NoError(t, err)
	got, err = enc.Evaluate(Request{Method: "GET", Path: "/p/a~b@c%2Fd:e"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "a~b@c/d:e", got.Allow.Vars["x"], "only %2F decodes; tilde, at, colon stay raw")

	// The encoded form of a non-separator char is rejected at tokenize, so it can
	// never be matched or forwarded. This is why a "~" never needs re-encoding:
	// an encoded "~" simply does not get in.
	set, err := CompileRoles([]Role{{Name: "r", Expressions: []string{
		`path.match(literal("p", glob_encoded(set("/"))))`,
	}}})
	require.NoError(t, err)
	for _, escape := range []string{"%7E", "%7e", "%40", "%3A", "%2E"} {
		got, err := set.Evaluate(Request{Method: "GET", Path: "/p/a" + escape + "b"}, Identity{})
		require.NoError(t, err)
		require.False(t, got.Allowed, escape)
		require.Equal(t, DenyInvalidRequest, got.Deny.Kind, "encoded %q must be rejected at tokenize", escape)
	}
}

// TestGlobEncodedEval pins the non-capturing encoded glob directly against the
// evaluator: it admits one segment that is plain or carries only the encoded
// separator, and rejects an empty segment. It binds nothing, the difference from
// capture_encoded.
func TestGlobEncodedEval(t *testing.T) {
	root := Literal("p", mustNode(GlobEncoded([]string{"/"})))
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/p/plain", true},
		{"/p/mygroup%2Fmyproject", true},
		{"/p/a%2fb", true},
		{"/p/a/b", false}, // two segments, the terminal glob matches one
	} {
		tokens, err := Tokenize(tc.path)
		require.NoError(t, err)
		ok, vars := Eval(tokens, root)
		require.Equal(t, tc.want, ok, tc.path)
		if ok {
			require.Empty(t, vars, "glob_encoded binds nothing")
		}
	}
}
