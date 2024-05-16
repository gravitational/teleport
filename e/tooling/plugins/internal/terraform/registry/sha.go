/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
