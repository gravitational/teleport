package nostructfieldassign

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

// TestAnalyzer runs integration tests via analysistest against fixture packages
// under testdata/src/. Each sub-test uses a distinct fixture package so that
// // want comments are unambiguous for the given analyzer configuration.
func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()

	t.Run("forbidden field", func(t *testing.T) {
		a := NewAnalyzer(Rule{
			Package: "example.com/mypkg",
			Type:    "MyStruct",
			Field:   "Forbidden",
		})
		analysistest.Run(t, testdata, a, "forbidden")
	})

	t.Run("custom message", func(t *testing.T) {
		a := NewAnalyzer(Rule{
			Package:      "example.com/mypkg",
			Type:         "MyStruct",
			Field:        "Forbidden",
			ErrorMessage: "use SetForbidden() instead",
		})
		analysistest.Run(t, testdata, a, "message")
	})

	t.Run("no fields configured", func(t *testing.T) {
		a := NewAnalyzer()
		analysistest.Run(t, testdata, a, "noconfig")
	})
}
