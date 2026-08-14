package structured

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema/schematypes"
)

func TestCommandAnalysisValidate(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, Validate[schematypes.CommandAnalysis](mustJSON(t, validCommandAnalysis())))
	})

	t.Run("missing required property", func(t *testing.T) {
		t.Parallel()

		p := validCommandAnalysis()
		delete(p, "category")

		require.ErrorContains(t, Validate[schematypes.CommandAnalysis](mustJSON(t, p)), `missing required property "category"`)
	})

	t.Run("empty object reports a missing property", func(t *testing.T) {
		t.Parallel()

		require.ErrorContains(t, Validate[schematypes.CommandAnalysis]([]byte(`{}`)), `missing required property "command"`)
	})

	t.Run("unexpected property is rejected", func(t *testing.T) {
		t.Parallel()

		p := validCommandAnalysis()
		p["extra"] = "nope"

		require.ErrorContains(t, Validate[schematypes.CommandAnalysis](mustJSON(t, p)), `property "extra" is not allowed`)
	})

	t.Run("server-only field is not a schema property", func(t *testing.T) {
		t.Parallel()

		// StartOffset is jsonschema:"-": the model never produces it, so its presence in a response is an unexpected property.
		p := validCommandAnalysis()
		p["StartOffset"] = 5

		require.ErrorContains(t, Validate[schematypes.CommandAnalysis](mustJSON(t, p)), `property "StartOffset" is not allowed`)
	})

	t.Run("invalid enum value", func(t *testing.T) {
		t.Parallel()

		p := validCommandAnalysis()
		p["risk_level"] = "extreme"

		require.ErrorContains(t, Validate[schematypes.CommandAnalysis](mustJSON(t, p)), "not one of the permitted values")
	})

	t.Run("wrong value types", func(t *testing.T) {
		t.Parallel()

		cases := map[string]struct {
			field string
			value any
			want  string
		}{
			"string as number":      {"command", 5, "response.command: expected string"},
			"boolean as string":     {"success", "yes", "response.success: expected boolean"},
			"integer as fraction":   {"risk_score", 5.5, "response.risk_score: expected integer"},
			"integer as string":     {"risk_score", "10", "response.risk_score: expected integer"},
			"array as string":       {"iocs", "x", "response.iocs: expected array"},
			"required string null":  {"command", nil, "response.command: expected string"},
			"array element as bool": {"iocs", []any{true}, "response.iocs[0]: expected string"},
		}
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				p := validCommandAnalysis()
				p[tc.field] = tc.value

				require.ErrorContains(t, Validate[schematypes.CommandAnalysis](mustJSON(t, p)), tc.want)
			})
		}
	})

	t.Run("integer accepts an integral float", func(t *testing.T) {
		t.Parallel()

		p := validCommandAnalysis()
		// json.RawMessage preserves the literal "10.0"; per JSON Schema this is a valid integer.
		p["risk_score"] = json.RawMessage("10.0")

		require.NoError(t, Validate[schematypes.CommandAnalysis](mustJSON(t, p)))
	})

	t.Run("malformed JSON", func(t *testing.T) {
		t.Parallel()

		require.Error(t, Validate[schematypes.CommandAnalysis]([]byte(`not json`)))
	})

	t.Run("non-object top level", func(t *testing.T) {
		t.Parallel()

		require.ErrorContains(t, Validate[schematypes.CommandAnalysis]([]byte(`[]`)), "response: expected object")
	})
}

func TestDesktopScreenshotAnalysisValidate(t *testing.T) {
	t.Parallel()

	t.Run("valid with nested events", func(t *testing.T) {
		t.Parallel()

		p := map[string]any{"notable_session_events": []any{validDesktopSessionEvent()}}

		require.NoError(t, Validate[schematypes.DesktopScreenshotAnalysis](mustJSON(t, p)))
	})

	t.Run("nested event missing required property", func(t *testing.T) {
		t.Parallel()

		event := validDesktopSessionEvent()
		delete(event, "category")
		p := map[string]any{"notable_session_events": []any{event}}

		require.ErrorContains(t, Validate[schematypes.DesktopScreenshotAnalysis](mustJSON(t, p)),
			`notable_session_events[0]: missing required property "category"`)
	})

	t.Run("nested event invalid enum reports its index", func(t *testing.T) {
		t.Parallel()

		bad := validDesktopSessionEvent()
		bad["category"] = "bogus"
		p := map[string]any{"notable_session_events": []any{validDesktopSessionEvent(), bad}}
		err := Validate[schematypes.DesktopScreenshotAnalysis](mustJSON(t, p))

		require.ErrorContains(t, err, "notable_session_events[1]")
		require.ErrorContains(t, err, "not one of the permitted values")
	})

	t.Run("nested event unexpected property", func(t *testing.T) {
		t.Parallel()

		event := validDesktopSessionEvent()
		event["surprise"] = true
		p := map[string]any{"notable_session_events": []any{event}}

		require.ErrorContains(t, Validate[schematypes.DesktopScreenshotAnalysis](mustJSON(t, p)),
			`notable_session_events[0]: property "surprise" is not allowed`)
	})

	t.Run("events field is not an array", func(t *testing.T) {
		t.Parallel()

		p := map[string]any{"notable_session_events": "nope"}

		require.ErrorContains(t, Validate[schematypes.DesktopScreenshotAnalysis](mustJSON(t, p)), "expected array")
	})
}

func TestSessionAnalysisValidate(t *testing.T) {
	t.Parallel()

	valid := func() map[string]any {
		return map[string]any{
			"short_description":       "summary",
			"session_description":     "details",
			"suspicious_activities":   []string{},
			"security_incidents":      []string{},
			"compromise_indicators":   false,
			"notable_command_indexes": []int{0, 2},
			"risk_level":              "none",
			"risk_score":              0,
		}
	}

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, Validate[schematypes.SessionAnalysis](mustJSON(t, valid())))
	})

	t.Run("integer array rejects a non-integer element", func(t *testing.T) {
		t.Parallel()

		p := valid()
		p["notable_command_indexes"] = []any{0, "two"}

		require.ErrorContains(t, Validate[schematypes.SessionAnalysis](mustJSON(t, p)),
			"notable_command_indexes[1]: expected integer")
	})
}

func TestDesktopSessionAnalysisValidate(t *testing.T) {
	t.Parallel()

	valid := map[string]any{
		"short_description":     "summary",
		"session_description":   "details",
		"suspicious_activities": []string{},
		"security_incidents":    []string{},
		"compromise_indicators": false,
		"risk_level":            "none",
		"risk_score":            0,
	}
	require.NoError(t, Validate[schematypes.DesktopSessionAnalysis](mustJSON(t, valid)))

	missing := map[string]any{
		"short_description":     "summary",
		"session_description":   "details",
		"suspicious_activities": []string{},
		"security_incidents":    []string{},
		"compromise_indicators": false,
		"risk_score":            0,
	}
	require.ErrorContains(t, Validate[schematypes.DesktopSessionAnalysis](mustJSON(t, missing)),
		`missing required property "risk_level"`)
}

func TestProseEmbeddingValidate(t *testing.T) {
	t.Parallel()

	require.NoError(t, Validate[schematypes.ProseEmbedding]([]byte(`{"condensed_text":"hello"}`)))
	require.ErrorContains(t, Validate[schematypes.ProseEmbedding]([]byte(`{}`)), `missing required property "condensed_text"`)
	require.ErrorContains(t, Validate[schematypes.ProseEmbedding]([]byte(`{"condensed_text":"hi","extra":1}`)),
		`property "extra" is not allowed`)
}

func TestValidateJoinsProblems(t *testing.T) {
	t.Parallel()

	// Two problems at once: condensed_text is missing and extra is not allowed.
	err := Validate[schematypes.ProseEmbedding]([]byte(`{"extra":1}`))
	require.Error(t, err)

	require.ErrorContains(t, err, `missing required property "condensed_text"`)
	require.ErrorContains(t, err, `property "extra" is not allowed`)
	require.ErrorContains(t, err, "; ")
	require.NotContains(t, err.Error(), "[response", "problems must not be rendered as a bracketed slice")
}

func TestValidateEngineEdgeCases(t *testing.T) {
	t.Parallel()

	type inner struct {
		Name string `json:"name" jsonschema:"required,enum=a,enum=b"`
	}
	type sample struct {
		Ratio   float64 `json:"ratio" jsonschema:"required"`
		Maybe   *string `json:"maybe"`
		Nested  inner   `json:"nested" jsonschema:"required"`
		Ignored string  `json:"-" jsonschema:"-"`
	}

	cases := []struct {
		name    string
		json    string
		wantErr string
	}{
		{"valid, optional pointer omitted", `{"ratio":1.5,"nested":{"name":"a"}}`, ""},
		{"integer satisfies number", `{"ratio":2,"nested":{"name":"b"}}`, ""},
		{"null satisfies optional pointer", `{"ratio":1.5,"maybe":null,"nested":{"name":"a"}}`, ""},
		{"string satisfies optional pointer", `{"ratio":1.5,"maybe":"x","nested":{"name":"a"}}`, ""},
		{"pointer element wrong type", `{"ratio":1.5,"maybe":5,"nested":{"name":"a"}}`, "response.maybe: expected string"},
		{"number field wrong type", `{"ratio":"x","nested":{"name":"a"}}`, "response.ratio: expected number"},
		{"nested enum violation", `{"ratio":1.5,"nested":{"name":"c"}}`, "response.nested.name"},
		{"ignored field rejected as extra", `{"ratio":1.5,"nested":{"name":"a"},"Ignored":"x"}`, `property "Ignored" is not allowed`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := Validate[sample]([]byte(tc.json))
			if tc.wantErr == "" {
				require.NoError(t, err)

				return
			}

			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestValidateTopLevelMustBeObject(t *testing.T) {
	t.Parallel()

	type sample struct {
		Name string `json:"name" jsonschema:"required"`
	}

	// A top-level null is rejected even when T is a pointer type: the response must be an object, not the
	// nullable-field case that checkValue's pointer branch otherwise allows.
	require.ErrorContains(t, Validate[*sample]([]byte("null")), "response: expected object")
	require.ErrorContains(t, Validate[sample]([]byte("null")), "response: expected object")

	// A valid object still passes through a pointer type parameter.
	require.NoError(t, Validate[*sample]([]byte(`{"name":"x"}`)))
}

func TestSchemaMatchesGenerated(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		typ  reflect.Type
		file string
	}{
		{"CommandAnalysis", reflect.TypeFor[schematypes.CommandAnalysis](), "command_analysis.json"},
		{"SessionAnalysis", reflect.TypeFor[schematypes.SessionAnalysis](), "session_analysis.json"},
		{"DesktopScreenshotAnalysis", reflect.TypeFor[schematypes.DesktopScreenshotAnalysis](), "desktop_screenshot_analysis.json"},
		{"DesktopSessionAnalysis", reflect.TypeFor[schematypes.DesktopSessionAnalysis](), "desktop_session_analysis.json"},
		{"ProseEmbedding", reflect.TypeFor[schematypes.ProseEmbedding](), "prose_embedding.json"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join("..", "schema", "generated", tc.file))
			require.NoError(t, err)

			require.Equal(t, schemaViewFromJSON(t, raw), schemaViewFromType(tc.typ, nil),
				"validate.go's schema for %s has drifted from generated/%s", tc.name, tc.file)
		})
	}
}

// schemaView is the subset of a JSON Schema that validate.go actually enforces.
type schemaView struct {
	Type         string
	Enum         []string
	Properties   map[string]schemaView
	Required     []string
	NoAdditional bool
	Items        *schemaView
}

// schemaViewFromType derives the schema validate.go enforces for a Go type, reusing fieldSchema  and the same Kind
// dispatch as checkValue.
func schemaViewFromType(t reflect.Type, enum []string) schemaView {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Struct:
		v := schemaView{Type: "object", NoAdditional: true, Properties: map[string]schemaView{}}
		for i := range t.NumField() {
			field := t.Field(i)
			name, required, fieldEnum, skip := fieldSchema(field)
			if skip {
				continue
			}

			v.Properties[name] = schemaViewFromType(field.Type, fieldEnum)
			if required {
				v.Required = append(v.Required, name)
			}
		}
		v.Required = sortedStrings(v.Required)

		return v

	case reflect.Slice, reflect.Array:
		item := schemaViewFromType(t.Elem(), nil)

		return schemaView{Type: "array", Items: &item}

	case reflect.String:
		return schemaView{Type: "string", Enum: sortedStrings(enum)}

	case reflect.Bool:
		return schemaView{Type: "boolean"}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return schemaView{Type: "integer"}

	case reflect.Float32, reflect.Float64:
		return schemaView{Type: "number"}

	default:
		return schemaView{Type: "unconstrained"}
	}
}

// schemaViewFromJSON parses the same enforced subset out of a generated JSON Schema document.
func schemaViewFromJSON(t *testing.T, raw []byte) schemaView {
	t.Helper()

	var doc struct {
		Type                 string                     `json:"type"`
		Enum                 []string                   `json:"enum"`
		Properties           map[string]json.RawMessage `json:"properties"`
		Required             []string                   `json:"required"`
		AdditionalProperties json.RawMessage            `json:"additionalProperties"`
		Items                json.RawMessage            `json:"items"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))

	v := schemaView{Type: doc.Type, Enum: sortedStrings(doc.Enum)}

	switch doc.Type {
	case "object":
		v.Properties = map[string]schemaView{}
		for name, prop := range doc.Properties {
			v.Properties[name] = schemaViewFromJSON(t, prop)
		}
		v.Required = sortedStrings(doc.Required)
		v.NoAdditional = strings.TrimSpace(string(doc.AdditionalProperties)) == "false"

	case "array":
		item := schemaViewFromJSON(t, doc.Items)
		v.Items = &item
	}

	return v
}

func sortedStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}

	out := slices.Clone(in)
	slices.Sort(out)

	return out
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()

	data, err := json.Marshal(v)
	require.NoError(t, err)

	return data
}

func validCommandAnalysis() map[string]any {
	return map[string]any{
		"command":              "ls -al",
		"category":             "file_operation",
		"success":              true,
		"risk_level":           "low",
		"risk_score":           10,
		"threat_category":      "none",
		"timeline_title":       "Listed directory contents",
		"timeline_subtitle":    "",
		"short_description":    "Listed all files",
		"description":          "Listed files in the directory",
		"error_messages":       []string{},
		"suspicious_flags":     []string{},
		"sensitive_items":      []string{},
		"suspicious_patterns":  []string{},
		"iocs":                 []string{},
		"mitre_attack_ids":     []string{}, //nolint:misspell // ignore MITRE
		"has_sensitive_data":   false,
		"privilege_escalation": false,
		"data_exfiltration":    false,
		"persistence":          false,
	}
}

func validDesktopSessionEvent() map[string]any {
	return map[string]any{
		"category":               "file_operation",
		"start_screenshot_index": 0,
		"end_screenshot_index":   1,
		"risk_level":             "none",
		"risk_score":             0,
		"threat_category":        "none",
		"timeline_title":         "Opened a file",
		"timeline_subtitle":      "",
		"short_description":      "Opened a file",
		"detailed_description":   "Opened a file in the editor",
		"suspicious_flags":       []string{},
		"sensitive_items":        []string{},
		"suspicious_patterns":    []string{},
		"iocs":                   []string{},
		"mitre_attack_ids":       []string{}, //nolint:misspell // ignore MITRE
		"has_sensitive_data":     false,
		"privilege_escalation":   false,
		"data_exfiltration":      false,
		"persistence":            false,
		"applications":           []string{"Editor"},
		"visible_urls":           []string{},
		"visible_file_paths":     []string{},
		"active_window_title":    "Editor",
	}
}
