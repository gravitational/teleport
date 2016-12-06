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

*/

package web

import (
	"archive/zip"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"github.com/kardianos/osext"
)

// relative path to static assets. this is useful during development.
var debugAssetsPath string

// NewStaticFileSystem returns the initialized implementation of http.FileSystem
// interface which can be used to serve Teleport Proxy Web UI
//
// If 'debugMode' is true, it will load the web assets from the same git repo
// directory where the executable is, otherwise it will load them from the embedded
// zip archive.
//
func NewStaticFileSystem(debugMode bool) (http.FileSystem, error) {
	if debugMode {
		assetsToCheck := []string{"index.html", "/app"}

		if debugAssetsPath == "" {
			exePath, err := osext.ExecutableFolder()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			debugAssetsPath = path.Join(exePath, "../web/dist")
		}

		for _, af := range assetsToCheck {
			_, err := os.Stat(filepath.Join(debugAssetsPath, af))
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		log.Infof("[Web] Using filesystem for serving web assets: %s", debugAssetsPath)
		return http.Dir(debugAssetsPath), nil
	}

	// otherwise, lets use the zip archive attached to the executable:
	return loadZippedExeAssets()
}

// isDebugMode determines if teleport is running in a "debug" mode.
// It looks at DEBUG environment variable
func isDebugMode() bool {
	v, err := strconv.ParseBool(os.Getenv("DEBUG"))
	return v && err == nil
}

// LoadWebResources returns a filesystem implementation compatible
// with http.Serve.
//
// The "filesystem" is served from a zip file attached at the end of
// the executable
//
func loadZippedExeAssets() (ResourceMap, error) {
	// open ourselves (teleport binary) for reading:
	// NOTE: the file stays open to serve future Read() requests
	myExe, err := osext.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return readZipArchive(myExe)
}

func readZipArchive(archivePath string) (ResourceMap, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// feed the binary into the zip reader and enumerate all files
	// found in the attached zip file:
	info, err := file.Stat()
	if err != nil {
		log.Fatal(err)
	}
	zreader, err := zip.NewReader(file, info.Size())
	if err != nil {
		// this often happens when teleport is launched without the web assets
		// zip file attached to the binary. for launching it in such mode
		// set DEBUG environment variable to 1
		log.Fatalf("Failed reading web assets from the binary. %v", err)
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
