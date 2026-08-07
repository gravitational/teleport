// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"debug/buildinfo"
	"runtime/debug"
	"testing"
)

func TestParseExpr(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantKey     string
		wantMatcher string // "" means presence check (nil matcher)
		wantErr     bool
	}{
		{
			name:    "presence only",
			input:   "GOARCH",
			wantKey: "GOARCH",
		},
		{
			name:        "exact match",
			input:       "GOARCH=amd64",
			wantKey:     "GOARCH",
			wantMatcher: "amd64",
		},
		{
			name:        "regexp match",
			input:       "GOARCH=/^amd64$/",
			wantKey:     "GOARCH",
			wantMatcher: "/^amd64$/",
		},
		{
			name:        "list match with exact inner",
			input:       "-tags=(fips)",
			wantKey:     "-tags",
			wantMatcher: "(fips)",
		},
		{
			name:        "list match with regexp inner",
			input:       "-tags=(/^fips$/)",
			wantKey:     "-tags",
			wantMatcher: "(/^fips$/)",
		},
		{
			name:        "value containing equals",
			input:       "DefaultGODEBUG=fips140=on",
			wantKey:     "DefaultGODEBUG",
			wantMatcher: "fips140=on",
		},
		{
			name:        "single slash is exact match",
			input:       "A=/",
			wantKey:     "A",
			wantMatcher: "/",
		},
		{
			name:        "empty parens is list match",
			input:       "A=()",
			wantKey:     "A",
			wantMatcher: "()",
		},
		{
			name:    "invalid regexp",
			input:   "GOARCH=/[invalid/",
			wantKey: "GOARCH",
			wantErr: true,
		},
		{
			name:    "invalid regexp in list",
			input:   "-tags=(/[invalid/)",
			wantKey: "-tags",
			wantErr: true,
		},
		{
			name:    "missing setting name",
			input:   "=value",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := parseExpr(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if expr.key != tt.wantKey {
				t.Errorf("key = %q, want %q", expr.key, tt.wantKey)
			}
			if tt.wantMatcher == "" && expr.matcher != nil {
				t.Errorf("expected nil matcher for presence check, got %s", expr.matcher)
			}
			if tt.wantMatcher != "" && expr.matcher == nil {
				t.Fatalf("expected matcher %q, got nil", tt.wantMatcher)
			}
			if tt.wantMatcher != "" && expr.matcher.String() != tt.wantMatcher {
				t.Errorf("matcher = %q, want %q", expr.matcher.String(), tt.wantMatcher)
			}
		})
	}
}

func TestExactMatcher(t *testing.T) {
	tests := []struct {
		name  string
		want  string
		value string
		match bool
	}{
		{"equal", "amd64", "amd64", true},
		{"not equal", "amd64", "arm64", false},
		{"empty value", "amd64", "", false},
		{"empty want", "", "", true},
		{"substring not exact", "amd", "amd64", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &exactMatcher{want: tt.want}
			if got := m.match(tt.value); got != tt.match {
				t.Errorf("match(%q) = %v, want %v", tt.value, got, tt.match)
			}
		})
	}
}

func TestRegexpMatcher(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		match   bool
	}{
		{"full match", "^amd64$", "amd64", true},
		{"no match", "^amd64$", "arm64", false},
		{"partial match", "amd", "amd64", true},
		{"alternation", "^(amd|arm)64$", "arm64", true},
		{"complex pattern", "fips140=on(ly)?", "fips140=on", true},
		{"complex pattern only", "fips140=on(ly)?", "fips140=only", true},
		{"complex pattern no match", "fips140=on(ly)?", "fips140=off", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := parseMatch("/" + tt.pattern + "/")
			if err != nil {
				t.Fatalf("parseMatch: %v", err)
			}
			if got := m.match(tt.value); got != tt.match {
				t.Errorf("match(%q) = %v, want %v", tt.value, got, tt.match)
			}
		})
	}
}

func TestListMatcher(t *testing.T) {
	tests := []struct {
		name  string
		expr  string
		value string
		match bool
	}{
		{"exact element match", "(fips)", "fips", true},
		{"exact in comma list", "(fips)", "boringcrypto,fips", true},
		{"exact not in list", "(fips)", "boringcrypto,other", false},
		{"regexp element match", "(/^fips$/)", "fips", true},
		{"regexp in comma list", "(/fips/)", "boringcrypto,fips,other", true},
		{"regexp no match", "(/^fips$/)", "boringcrypto,other", false},
		{"single element list", "(foo)", "foo", true},
		{"single element no match", "(foo)", "bar", false},
		{"empty value", "(foo)", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := parseMatch(tt.expr)
			if err != nil {
				t.Fatalf("parseMatch: %v", err)
			}
			if got := m.match(tt.value); got != tt.match {
				t.Errorf("match(%q) = %v, want %v", tt.value, got, tt.match)
			}
		})
	}
}

func TestVerify(t *testing.T) {
	info := &buildinfo.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "GOARCH", Value: "amd64"},
			{Key: "CGO_ENABLED", Value: "1"},
			{Key: "-tags", Value: "fips,boringcrypto"},
			{Key: "GOFIPS140", Value: "v1.0.0-c2097c7c"},
		},
	}

	tests := []struct {
		name     string
		exprs    []string
		wantErrs []string
	}{
		{
			name:  "all pass",
			exprs: []string{"GOARCH=amd64", "CGO_ENABLED"},
		},
		{
			name:  "setting not present",
			exprs: []string{"MISSING"},
			wantErrs: []string{
				"Build setting not present: MISSING",
			},
		},
		{
			name:  "setting not present for matcher",
			exprs: []string{"MISSING=value"},
			wantErrs: []string{
				"Build setting not present: MISSING",
			},
		},
		{
			name:  "exact mismatch",
			exprs: []string{"GOARCH=arm64"},
			wantErrs: []string{
				"Build setting value does not match: GOARCH: amd64 != arm64",
			},
		},
		{
			name:  "regexp mismatch",
			exprs: []string{"GOARCH=/^arm/"},
			wantErrs: []string{
				"Build setting value does not match: GOARCH: amd64 != /^arm/",
			},
		},
		{
			name:  "list match",
			exprs: []string{"-tags=(fips)"},
		},
		{
			name:  "list no match",
			exprs: []string{"-tags=(nothere)"},
			wantErrs: []string{
				"Build setting does not contain value: -tags: nothere not in fips,boringcrypto",
			},
		},
		{
			name:  "multiple errors",
			exprs: []string{"MISSING", "GOARCH=arm64"},
			wantErrs: []string{
				"Build setting not present: MISSING",
				"Build setting value does not match: GOARCH: amd64 != arm64",
			},
		},
		{
			name:  "presence only passes",
			exprs: []string{"GOARCH"},
		},
		{
			name:  "invalid expression",
			exprs: []string{"GOARCH=/[bad/"},
			wantErrs: []string{
				"invalid regexp /[bad/: error parsing regexp: missing closing ]: `[bad`",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := verify(info, tt.exprs)
			if len(errs) != len(tt.wantErrs) {
				t.Fatalf("got %d errors, want %d:\n  got:  %v\n  want: %v", len(errs), len(tt.wantErrs), errs, tt.wantErrs)
			}
			for i := range errs {
				if errs[i].Error() != tt.wantErrs[i] {
					t.Errorf("error[%d] = %q, want %q", i, errs[i].Error(), tt.wantErrs[i])
				}
			}
		})
	}
}
