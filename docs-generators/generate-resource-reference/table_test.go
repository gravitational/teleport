package main

import (
	"io"
	"testing"

	"github.com/gravitational/teleport/schemagen"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestGenerateTable(t *testing.T) {
	cases := []struct {
		description string
		// For convenience, test cases parse YAML documents as
		// schemagen.SchemaCollections. These must comply with the OpenAPI V3
		// JSON Schema format unless we expect an error.
		inputYAML       io.Reader
		expectedOutputs []string
		// Substring within the expected error message. If blank, we don't
		// expect an error.
		errSubstring string
	}{}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var sc schemagen.SchemaCollection
			if err := yaml.NewDecoder(c.inputYAML).Decode(&sc); err != nil {
				t.Fatalf("error decoding input YAML when setting up the test: %v", err)
			}

			actual, err := generateTable(&sc)

			if c.errSubstring != "" {
				assert.Contains(t, err.Error(), c.errSubstring)
			}

			assert.ElementsMatch(t, c.expectedOutputs, actual)

		})
	}
}
