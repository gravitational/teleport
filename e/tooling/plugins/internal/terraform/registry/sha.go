package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"

	"github.com/gravitational/trace"
)

// sha256Sum generates the equivalent contents of running `sha256sum` on some
// data and writes it to the supplied stream, returning the sha bytes.
func sha256Sum(dst io.Writer, filename string, data io.Reader) ([]byte, error) {
	hasher := sha256.New()
	_, err := io.Copy(hasher, data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sha := hasher.Sum(make([]byte, 0, sha256.Size))

	_, err = fmt.Fprintf(dst, "%s  %s\n", hex.EncodeToString(sha[:]), filepath.Base(filename))
	return sha, trace.Wrap(err, "failed generating sum file")
}
