/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This file contains utilities to load web resources (static files
like HTML, JavaScript and CSS) from the executable instead of
the file system.

*/

package web

import (
	"archive/zip"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"github.com/kardianos/osext"
)

const (
	// relative path to static assets. this is useful during development:
	debugAssetsPath = "../web/dist"

	// useDebugPath constant, if set to true, allows Teleport to seek
	// static assets in debugAssetsPath (relative to its executable).
	useDebugPath = true
)

// NewStaticFileSystem returns the initialized implementation of http.FileSystem
// interface which can be used to serve Teleport Proxy Web UI
func NewStaticFileSystem() (http.FileSystem, error) {
	useLocalDisk := useDebugPath
	assetsToCheck := []string{"index.html", "/app"}

	// shall we look for web assets in the "debug assets path", i.e. the
	// path relative to the executable, as laid out in the git repo?
	if useLocalDisk {
		exePath, err := osext.ExecutableFolder()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		dp := path.Join(exePath, debugAssetsPath)

		for _, af := range assetsToCheck {
			_, err := os.Stat(filepath.Join(dp, af))
			if os.IsNotExist(err) {
				useLocalDisk = false
				break
			}
		}
		if useLocalDisk {
			log.Infof("[Web] Using filesystem for serving web assets: %s", dp)
			return http.Dir(dp), nil
		}
	}

	// otherwise, lets use the zip archive attached to the executable:
	return loadZippedExeAssets()
}

// LoadWebResources returns a filesystem implementation compatible
// with http.Serve. The "filesystem" is served from a zip file attached
// at the end of the executable
func loadZippedExeAssets() (ResourceMap, error) {
	// open ourselves (teleport binary) for reading:
	myExe, err := osext.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	file, err := os.Open(myExe)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// defer file.Close()
	// feed the binary into the zip reader and enumerate all files
	// found in the attached zip file:
	info, err := file.Stat()
	if err != nil {
		log.Fatal(err)
	}
	zreader, err := zip.NewReader(file, info.Size())
	if err != nil {
		log.Fatal(err)
	}
	entries := make(ResourceMap)
	for _, file := range zreader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		entries[file.Name] = file
	}
	// no entries found?
	if len(entries) == 0 {
		return nil, trace.Wrap(os.ErrInvalid)
	}
	return entries, nil
}

// resource struct implements http.File interface on top of zip.File object
type resource struct {
	reader io.ReadCloser
	file   *zip.File
}

func (rsc *resource) Read(p []byte) (n int, err error) {
	return rsc.reader.Read(p)
}

func (rsc *resource) Seek(offset int64, whence int) (int64, error) {
	return offset, nil
}

func (rsc *resource) Readdir(count int) ([]os.FileInfo, error) {
	return nil, trace.Wrap(os.ErrPermission)
}

func (rsc *resource) Stat() (os.FileInfo, error) {
	return rsc.file.FileInfo(), nil
}

func (rsc *resource) Close() (err error) {
	log.Debugf("[web] zip::Close(%s)", rsc.file.FileInfo().Name())
	return rsc.reader.Close()
}

type ResourceMap map[string]*zip.File

func (rm ResourceMap) Open(name string) (http.File, error) {
	log.Debugf("[web] GET zip:%s", name)
	f, ok := rm[strings.Trim(name, "/")]
	if !ok {
		return nil, trace.Wrap(os.ErrNotExist)
	}
	reader, err := f.Open()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &resource{reader, f}, nil
}
