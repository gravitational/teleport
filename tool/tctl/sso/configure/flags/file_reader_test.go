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
	"math/rand"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFileReader(t *testing.T) {
	out := "initial"
	reader := NewFileReader(&out)

	tmp := t.TempDir()

	// running against non-existing file returns error, does not change the stored value
	fn := path.Join(tmp, "does-not-exist.txt")
	err := reader.Set(fn)
	require.Error(t, err)
	require.Equal(t, "initial", out)

	// lots of ones...
	fn = path.Join(tmp, "ones.txt")
	ones := strings.Repeat("1", 1024*1024)
	err = os.WriteFile(fn, []byte(ones), 0777)
	require.NoError(t, err)
	err = reader.Set(fn)
	require.NoError(t, err)
	require.Equal(t, ones, out)

	// random string
	fn = path.Join(tmp, "random.txt")
	src := rand.NewSource(time.Now().UnixNano())
	buf := make([]byte, 1024*1024)
	for ix := range buf {
		buf[ix] = byte(src.Int63())
	}
	err = os.WriteFile(fn, buf, 0777)
	require.NoError(t, err)
	err = reader.Set(fn)
	require.NoError(t, err)
	require.Equal(t, buf, []byte(out))
}
