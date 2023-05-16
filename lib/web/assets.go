/*
Copyright 2021 Gravitational, Inc.

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
