package nostructfieldassign

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestParseFieldKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    fieldKey
		wantErr bool
	}{
		{
			name:  "simple package path",
			input: "example.com/pkg.MyType.MyField",
			want:  fieldKey{pkgPath: "example.com/pkg", typeName: "MyType", fieldName: "MyField"},
		},
		{
			name:  "nested package path",
			input: "github.com/aws/aws-sdk-go-v2/aws.Config.Region",
			want:  fieldKey{pkgPath: "github.com/aws/aws-sdk-go-v2/aws", typeName: "Config", fieldName: "Region"},
		},
		{
			name:  "with message",
			input: "example.com/pkg.MyType.MyField# use SetMyField() instead",
			want:  fieldKey{pkgPath: "example.com/pkg", typeName: "MyType", fieldName: "MyField", msg: "use SetMyField() instead"},
		},
		{
			name:  "message without leading space",
			input: "example.com/pkg.MyType.MyField#no leading space",
			want:  fieldKey{pkgPath: "example.com/pkg", typeName: "MyType", fieldName: "MyField", msg: "no leading space"},
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
			wantErr: true,
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
			got, err := parseFieldKey(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseFieldKey(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Errorf("parseFieldKey(%q)\n got  %+v\n want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFieldsFlagRoundTrip(t *testing.T) {
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
			var ff fieldsFlag
			if err := ff.Set(tt.input); err != nil {
				t.Fatalf("Set(%q) error: %v", tt.input, err)
			}
			if got := ff.String(); got != tt.input {
				t.Errorf("String() = %q, want %q", got, tt.input)
			}
		})
	}
}

func TestFieldsFlagSkipsEmpty(t *testing.T) {
	var ff fieldsFlag
	if err := ff.Set("  ,  ,  "); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	if len(ff) != 0 {
		t.Errorf("expected 0 fields, got %d", len(ff))
	}
}

func TestFieldsFlagRejectsInvalid(t *testing.T) {
	var ff fieldsFlag
	if err := ff.Set("invalid"); err == nil {
		t.Error("expected error for invalid field spec, got nil")
	}
}

// TestAnalyzer runs integration tests via analysistest against fixture packages
// under testdata/src/. Each sub-test uses a distinct fixture package so that
// // want comments are unambiguous for the given analyzer configuration.
func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()

	// "forbidden" package: analyzer configured with Forbidden field, no custom message.
	// Covers direct assignment, pointer receiver, composite literal, and allowed fields.
	t.Run("forbidden field", func(t *testing.T) {
		a := NewAnalyzer("example.com/mypkg.MyStruct.Forbidden")
		analysistest.Run(t, testdata, a, "forbidden")
	})

	// "message" package: same field but with a custom message in the spec.
	t.Run("custom message", func(t *testing.T) {
		a := NewAnalyzer("example.com/mypkg.MyStruct.Forbidden# use SetForbidden() instead")
		analysistest.Run(t, testdata, a, "message")
	})

	// "noconfig" package: no fields configured — nothing should be reported.
	t.Run("no fields configured", func(t *testing.T) {
		a := NewAnalyzer()
		analysistest.Run(t, testdata, a, "noconfig")
	})
}
