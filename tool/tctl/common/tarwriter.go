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
	"sort"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/client/identityfile"
)

// tarWriter implements a ConfigWriter that generates a tarfile from the
// files written to it.
type tarWriter struct {
	*identityfile.InMemoryConfigWriter
}

// newTarWriter creates a new tarWriter that caches the files written to it and
// dumps them to a tarball on demand.
func newTarWriter(clock clockwork.Clock) *tarWriter {
	cache := identityfile.NewInMemoryConfigWriter(identityfile.WithClock(clock))
	return &tarWriter{InMemoryConfigWriter: cache}
}

// Archive dumps the contents of the ConfigWriter to the supplied sink as
// a tarball. May be called multiple times on the same instance.
func (t *tarWriter) Archive(out io.Writer) error {
	tarball := tar.NewWriter(out)

	err := t.WithReadonlyFiles(func(files identityfile.InMemoryFS) error {
		// Sort the filenames so that files will be written to the output tarball
		// in a repeatable order
		filenames := make([]string, 0, len(files))
		for filename := range files {
			filenames = append(filenames, filename)
		}
		sort.Strings(filenames)

		// Stream the tarball to the supplied output writer
		for _, filename := range filenames {
			file := files[filename]
			header := &tar.Header{
				Name:    filename,
				Mode:    int64(file.Mode()),
				ModTime: file.ModTime(),
				Size:    file.Size(),
			}
			if err := tarball.WriteHeader(header); err != nil {
				return trace.Wrap(err)
			}
			if _, err := tarball.Write(file.Content()); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})

	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(tarball.Close())
}

// compile-time assertion that the tarWriter implements the ConfigWriter
// interface
var _ identityfile.ConfigWriter = (*tarWriter)(nil)
