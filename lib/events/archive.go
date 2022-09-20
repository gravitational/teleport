/*
Copyright 2017 Gravitational, Inc.

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

package events

import (
	"archive/tar"
	"io"

	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewSessionArchive returns generated tar archive with all components
func NewSessionArchive(dataDir, serverID, namespace string, sessionID session.ID) (io.ReadCloser, error) {
	index, err := readSessionIndex(
		dataDir, []string{serverID}, namespace, sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// io.Pipe allows to generate the archive part by part
	// without writing to disk or generating it in memory
	// at the pace which reader is ready to consume it
	reader, writer := io.Pipe()
	tarball := tar.NewWriter(writer)
	go func() {
		if err := writeSessionArchive(index, tarball, writer); err != nil {
			log.Warningf("Failed to write archive: %v.", trace.DebugReport(err))
		}
	}()

	return reader, nil
}

func writeTarFile(w *tar.Writer, header *tar.Header, reader io.ReadCloser) error {
	defer reader.Close()
	if err := w.WriteHeader(header); err != nil {
		return trace.Wrap(err)
	}
	_, err := io.Copy(w, reader)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func writeSessionArchive(index *sessionIndex, tarball *tar.Writer, writer *io.PipeWriter) error {
	defer writer.Close()
	defer tarball.Close()

	fileNames := index.fileNames()

	for _, fileName := range fileNames {
		header, reader, err := openFileForTar(fileName)
		if err != nil {
			writer.CloseWithError(err)
			return trace.Wrap(err)
		}
		if err := writeTarFile(tarball, header, reader); err != nil {
			writer.CloseWithError(err)
			return trace.Wrap(err)
		}
	}

	return nil
}
