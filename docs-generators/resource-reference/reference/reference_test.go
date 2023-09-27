package reference

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerate(t *testing.T) {
	// TODO: Read a golden file instead to get the expected value
	var expected string
	conf := GeneratorConfig{
		RequiredTypes: []TypeInfo{
			{
				Package: "typestest",
				Name:    "ResourceHeader",
			},
			{
				Package: "typestest",
				Name:    "Metadata",
			},
		},
		SourcePath: "testdata",
		// No-op in this case
		DestinationPath: "",
	}

	var buf bytes.Buffer
	assert.NoError(t, Generate(&buf, conf))
	assert.Equal(t, expected, buf.String())
}
