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
			description:  "simple case with only scalars",
			inputYAML:    mustReadFile(t, path.Join("testdata", "user-scalars.yaml")),
			expectedName: "user.yaml",
			expectedContent: strings.ReplaceAll(`|Property|Description|Type|
|---|---|---|
|~spec~|Options for configuring the user resource.|object|
|~spec.role~|The one role assigned to the user in this test|string|
|~spec.login~|The one login assigned to the user in this test|string|

spec:
  login: string
  role: string
`, "~", "`"),
		},
		{
			description:  "simple case with arrays of strings",
			inputYAML:    mustReadFile(t, path.Join("testdata", "user.yaml")),
			expectedName: "user.yaml",
			expectedContent: strings.ReplaceAll(`
|Property|Description|Type|
|---|---|---|
|~spec~|Options for configuring the user resource.|object|
|~spec.roles~|Roles is a list of roles assigned to the user|array[string]|
|~spec.traits~|Traits are key/value pairs received from an identity provider (through OIDC claims or SAML assertions) or from a system administrator for local accounts. Traits are used to populate role variables.|map[string]array[string]|

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
		{
			description:  "multiple versions",
			inputYAML:    mustReadFile(t, path.Join("testdata", "user-scalars-multiversion.yaml")),
			expectedName: "user.yaml",
			expectedContent: strings.ReplaceAll(`<Tabs>
<TabItem label="v2">
|Property|Description|Type|
|---|---|---|
|~spec~|Options for configuring the user resource.|object|
|~spec.role~|The one role assigned to the user in this test|string|
|~spec.login~|The one login assigned to the user in this test|string|

spec:
  role: "example"
  login: "example"
</TabItem>
<TabItem label="v3">
|Property|Description|Type|
|---|---|---|
|~spec~|Options for configuring the user resource.|object|
|~spec.role~|The one role assigned to the user in this test|string|
|~spec.login~|The one login assigned to the user in this test|string|

spec:
  role: "example"
  login: "example"
</TabItem>
</Tabs>
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
			} else if err != nil {
				t.Fatalf("unexpected error generating the configuration resource docs: %v", err)
			}

			if actual == nil {
				t.Fatal("got a nil TransformedFile")
			}

			assert.Equal(t, c.expectedName, actual.Name)
			assert.Equal(t, c.expectedContent, actual.Content)
		})
	}
}
