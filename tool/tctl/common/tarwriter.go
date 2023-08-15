// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"archive/tar"
	"io"
	"io/fs"
	"os"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/client/identityfile"
)

// tarWriter implements a ConfigWriter that generates a tarfile from the
// files written to the config writer. Does not implement
type tarWriter struct {
	tarball *tar.Writer
	clock   clockwork.Clock
}

// newTarWriter creates a new tarWriter that writes the generated tar
// file to the supplied `io.Writer`. Be sure to terminate the tar archive
// by calling `Close()` on the resuting `tarWriter`.
func newTarWriter(out io.Writer, clock clockwork.Clock) *tarWriter {
	return &tarWriter{
		tarball: tar.NewWriter(out),
		clock:   clock,
	}
}

// Remove is not implemented, and only exists to fill out the
// `ConfigWriter` interface.
func (t *tarWriter) Remove(_ string) error {
	return trace.NotImplemented("tarWriter.Remove()")
}

// Stat always returns `ErrNotExist` in ordre to sidestep the
// overwite check when writing certificates via a ConfigWriter.
func (t *tarWriter) Stat(_ string) (fs.FileInfo, error) {
	return nil, os.ErrNotExist
}

// WriteFile adds the supplied content to the tar archive.
func (t *tarWriter) WriteFile(name string, content []byte, mode fs.FileMode) error {
	header := &tar.Header{
		Name:    name,
		Mode:    int64(mode),
		ModTime: t.clock.Now(),
		Size:    int64(len(content)),
	}
	if err := t.tarball.WriteHeader(header); err != nil {
		return trace.Wrap(err)
	}
	if _, err := t.tarball.Write(content); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Close finalizes the tar archive, adding any necessary padding and footers.
func (t *tarWriter) Close() error {
	return trace.Wrap(t.tarball.Close())
}

// identityfile.ConfigWriter implementation check
var _ identityfile.ConfigWriter = (*tarWriter)(nil)
