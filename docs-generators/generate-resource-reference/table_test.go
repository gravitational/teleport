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
|~spec.login~|The one login assigned to the user in this test|string|
|~spec.role~|The one role assigned to the user in this test|string|

spec:
  login: string
  role: string
`, "~", "`"),
		},
		{
			description:  "simple case with arrays of scalars",
			inputYAML:    mustReadFile(t, path.Join("testdata", "user-array-scalars.yaml")),
			expectedName: "user.yaml",
			expectedContent: strings.ReplaceAll(`|Property|Description|Type|
|---|---|---|
|~spec~|Options for configuring the user resource.|object|
|~spec.booleans~|Booleans that the user has chosen|array[boolean]|
|~spec.lucky_numbers~|The lucky numbers assigned to the user in this test|array[number]|
|~spec.roles~|The roles assigned to the user in this test|array[string]|

spec:
  booleans:
  - false
  - true
  - false
  lucky_numbers:
  - 1
  - 2
  - 3
  roles:
  - string1
  - string2
  - string3
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
|~spec.login~|The one login assigned to the user in this test|string|
|~spec.role~|The one role assigned to the user in this test|string|

spec:
  login: string
  role: string

</TabItem>
<TabItem label="v3">
|Property|Description|Type|
|---|---|---|
|~spec~|Options for configuring the user resource.|object|
|~spec.role~|The one role assigned to the user in this test|string|
|~spec.tenure~|Number of years the user has been with the organization|number|

spec:
  role: string
  tenure: 0

</TabItem>
</Tabs>`, "~", "`"),
		},

		// TODO: Example with complex nested types, e.g., an "allow" rule
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
