//go:build linux

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package update

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func replace(path string, _ string) error {
	dir, err := toolsDir()
	if err != nil {
		return trace.Wrap(err)
	}
	tempDir := renameio.TempDir(dir)

	f, err := os.Open(path)
	if err != nil {
		return trace.Wrap(err)
	}

	gzipReader, err := gzip.NewReader(f)
	if err != nil {
		return trace.Wrap(err)
	}

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		// Skip over any files in the archive that are not {tsh, tctl}.
		if header.Name != "teleport/tctl" &&
			header.Name != "teleport/tsh" &&
			header.Name != "teleport/tbot" {
			if _, err := io.Copy(io.Discard, tarReader); err != nil {
				log.Debugf("failed to discard %v: %v.", header.Name, err)
			}
			continue
		}

		dest := filepath.Join(dir, strings.TrimPrefix(header.Name, "teleport/"))
		t, err := renameio.TempFile(tempDir, dest)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := os.Chmod(t.Name(), 0755); err != nil {
			return trace.Wrap(err)
		}
		defer t.Cleanup()

		if _, err := io.Copy(t, tarReader); err != nil {
			return trace.Wrap(err)
		}
		if err := t.CloseAtomicallyReplace(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
