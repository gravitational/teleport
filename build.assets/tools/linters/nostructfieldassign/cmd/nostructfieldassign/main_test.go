package main

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"
	"github.com/google/go-cmp/cmp"
	nostructfieldassign "github.com/gravitational/teleport/build.assets/tools/linters/nostructfieldassign"
)

func TestParseFieldKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ruleFlag
		wantErr bool
	}{
		{
			name:  "simple package path",
			input: "example.com/pkg.MyType.MyField",
			want: []nostructfieldassign.Rule{
				{
					Package: "example.com/pkg",
					Type:    "MyType",
					Field:   "MyField",
				},
			},
		},
		{
			name:  "nested package path",
			input: "github.com/aws/aws-sdk-go-v2/aws.Config.Region",
			want: []nostructfieldassign.Rule{
				{
					Package: "github.com/aws/aws-sdk-go-v2/aws",
					Type:    "Config",
					Field:   "Region",
				},
			},
		},
		{
			name:  "with message",
			input: "example.com/pkg.MyType.MyField# use SetMyField() instead",
			want: []nostructfieldassign.Rule{
				{
					Package:      "example.com/pkg",
					Type:         "MyType",
					Field:        "MyField",
					ErrorMessage: "use SetMyField() instead",
				},
			},
		},
		{
			name:  "with message including dot",
			input: "example.com/pkg.MyType.MyField#use SetMyField(). not direct assignment,example.com/pkg.MyType.MyField2#use SetMyField2(). not direct assignment",
			want: []nostructfieldassign.Rule{
				{
					Package:      "example.com/pkg",
					Type:         "MyType",
					Field:        "MyField",
					ErrorMessage: "use SetMyField(). not direct assignment",
				},
				{
					Package:      "example.com/pkg",
					Type:         "MyType",
					Field:        "MyField2",
					ErrorMessage: "use SetMyField2(). not direct assignment",
				},
			},
		},
		{
			name:  "with message including comman",
			input: "example.com/pkg.MyType.MyField#use SetMyField(), not direct assignment,example.com/pkg.MyType.MyField2#use SetMyField2(), not direct assignment",
			want: []nostructfieldassign.Rule{
				{
					Package:      "example.com/pkg",
					Type:         "MyType",
					Field:        "MyField",
					ErrorMessage: "use SetMyField(), not direct assignment",
				},
				{
					Package:      "example.com/pkg",
					Type:         "MyType",
					Field:        "MyField2",
					ErrorMessage: "use SetMyField2(), not direct assignment",
				},
			},
		},
		{
			name:  "message without leading space",
			input: "example.com/pkg.MyType.MyField#no leading space",
			want: []nostructfieldassign.Rule{
				{
					Package:      "example.com/pkg",
					Type:         "MyType",
					Field:        "MyField",
					ErrorMessage: "no leading space",
				},
			},
		},
		{
			name:    "only one component",
			input:   "NoPackage",
			wantErr: true,
		},
		{
			name:    "only two components",
			input:   "pkg.Type",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "empty pkg path",
			input:   ".Type.Field",
			wantErr: true,
		},
		{
			name:    "empty type name",
			input:   "example.com/pkg..Field",
			wantErr: true,
		},
		{
			name:    "empty field name",
			input:   "example.com/pkg.Type.",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rules ruleFlag

			err := rules.Set(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseFieldKey(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			require.Empty(t, cmp.Diff(rules, tt.want))
		})
	}
}

func TestRuleFlagRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "single field",
			input: "example.com/pkg.TypeA.FieldA",
		},
		{
			name:  "comma-separated fields",
			input: "example.com/pkg.TypeA.FieldA,example.com/pkg.TypeB.FieldB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rf ruleFlag
			if err := rf.Set(tt.input); err != nil {
				t.Fatalf("Set(%q) error: %v", tt.input, err)
			}
			if got := rf.String(); got != tt.input {
				t.Errorf("String() = %q, want %q", got, tt.input)
			}
		})
	}
}

func TestRuleFlagSkipsEmpty(t *testing.T) {
	var rf ruleFlag
	if err := rf.Set("  ,  ,  "); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	if len(rf) != 0 {
		t.Errorf("expected 0 fields, got %d", len(rf))
	}
}

func TestRuleFlagRejectsInvalid(t *testing.T) {
	var rf ruleFlag
	if err := rf.Set("invalid"); err == nil {
		t.Error("expected error for invalid field spec, got nil")
	}
}
