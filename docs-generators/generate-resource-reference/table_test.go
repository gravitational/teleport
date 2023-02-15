package main

import (
	"io"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/gravitational/teleport/schemagen"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func mustReadFile(t *testing.T, path string) io.Reader {
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("could not open the test data file: %v", err)
	}
	return f
}

func TestGenerateTable(t *testing.T) {
	cases := []struct {
		description string
		// For convenience, test cases parse YAML documents as
		// schemagen.RootSchemas.
		inputYAML       io.Reader
		expectedContent string
		expectedName    string
		// Substring within the expected error message. If blank, we don't
		// expect an error.
		errSubstring string
	}{
		{
			description:  "simple case",
			inputYAML:    mustReadFile(t, path.Join("testdata", "user.yaml")),
			expectedName: "user.yaml",
			expectedContent: strings.ReplaceAll(`
|Property|Description|Type|
|---|---|---|
|~spec~|Options for configuring the user resource.|object|
|~spec.roles~|Roles is a list of roles assigned to the user|array|
|~spec.traits~|Traits are key/value pairs received from an identity provider (through OIDC claims or SAML assertions) or from a system administrator for local accounts. Traits are used to populate role variables.|object (values are arrays of strings)|

spec:
  roles:
   - "example1"
   - "example2"
   - "example3"
  traits:
    key1:
      - "example1"
      - "example2"
      - "example3"
    key2:
      - "example1"
      - "example2"
      - "example3"
`, "~", "`"),
		},

		// TODO: Example with nested types, e.g., an "allow" rule
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var root schemagen.RootSchema
			if err := yaml.NewDecoder(c.inputYAML).Decode(&root); err != nil {
				t.Fatalf("error decoding input YAML when setting up the test: %v", err)
			}

			actual, err := generateTable(&root)

			if c.errSubstring != "" {
				assert.Contains(t, err.Error(), c.errSubstring)
			}

			if actual == nil {
				t.Fatal("got a nil TransformedFile")
			}

			assert.Equal(t, c.expectedName, actual.Name)
			assert.Equal(t, c.expectedContent, actual.Content)
		})
	}
}
