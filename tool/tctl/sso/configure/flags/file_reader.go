// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
