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
