package flags

import (
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

// flagFileReader implements kingpin.Value.
type flagFileReader struct {
	bytes []byte
	field *string
}

func (reader *flagFileReader) String() string {
	return string(reader.bytes)
}

func (reader *flagFileReader) Set(filename string) error {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return trace.Wrap(err)
	}
	reader.bytes = bytes
	*reader.field = string(bytes)
	return nil
}

// NewFileReader returns a file which will read the provided file and store the contents into provided field.
func NewFileReader(field *string) kingpin.Value {
	return &flagFileReader{field: field}
}
