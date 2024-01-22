/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package web

import (
	"archive/zip"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gravitational/trace"
)

// resource struct implements http.File interface on top of zip.File object
type resource struct {
	reader io.ReadCloser
	file   *zip.File
	pos    int64
}

func (rsc *resource) Read(p []byte) (n int, err error) {
	n, err = rsc.reader.Read(p)
	rsc.pos += int64(n)
	return n, err
}

func (rsc *resource) Seek(offset int64, whence int) (int64, error) {
	var (
		pos int64
		err error
	)
	// zip.File does not support seeking. To implement Seek on top of it,
	// we close the existing reader, re-open it, and read 'offset' bytes from
	// the beginning
	if err = rsc.reader.Close(); err != nil {
		return 0, err
	}
	if rsc.reader, err = rsc.file.Open(); err != nil {
		return 0, err
	}
	switch whence {
	case io.SeekStart:
		pos = offset
	case io.SeekCurrent:
		pos = rsc.pos + offset
	case io.SeekEnd:
		pos = int64(rsc.file.UncompressedSize64) + offset
	}
	if pos > 0 {
		b := make([]byte, pos)
		if _, err = rsc.reader.Read(b); err != nil {
			return 0, err
		}
	}
	rsc.pos = pos
	return pos, nil
}

func (rsc *resource) Readdir(count int) ([]os.FileInfo, error) {
	return nil, trace.Wrap(os.ErrPermission)
}

func (rsc *resource) Stat() (os.FileInfo, error) {
	return rsc.file.FileInfo(), nil
}

func (rsc *resource) Close() (err error) {
	log.Debugf("zip::Close(%s).", rsc.file.FileInfo().Name())
	return rsc.reader.Close()
}

type ResourceMap map[string]*zip.File

func (rm ResourceMap) Open(name string) (http.File, error) {
	log.Debugf("GET zip:%s.", name)
	f, ok := rm[strings.Trim(name, "/")]
	if !ok {
		return nil, trace.Wrap(os.ErrNotExist)
	}
	reader, err := f.Open()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &resource{
		reader: reader,
		file:   f,
	}, nil
}
