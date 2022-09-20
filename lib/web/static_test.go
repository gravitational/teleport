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
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

type StaticSuite struct {
}

var _ = check.Suite(&StaticSuite{})

func (s *StaticSuite) TestLocalFS(c *check.C) {
	fs, err := NewDebugFileSystem("../../webassets/teleport")
	c.Assert(err, check.IsNil)
	c.Assert(fs, check.NotNil)

	f, err := fs.Open("/index.html")
	c.Assert(err, check.IsNil)
	bytes, err := ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)

	html := string(bytes[:])
	c.Assert(f.Close(), check.IsNil)
	c.Assert(strings.Contains(html, `<script src="/web/config.js"></script>`), check.Equals, true)
	c.Assert(strings.Contains(html, `content="{{ .XCSRF }}"`), check.Equals, true)
}

func (s *StaticSuite) TestZipFS(c *check.C) {
	fs, err := readZipArchiveAt("../../fixtures/assets.zip")
	c.Assert(err, check.IsNil)
	c.Assert(fs, check.NotNil)

	// test simple full read:
	f, err := fs.Open("/index.html")
	c.Assert(err, check.IsNil)
	bytes, err := ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(len(bytes), check.Equals, 813)
	c.Assert(f.Close(), check.IsNil)

	// seek + read
	f, err = fs.Open("/index.html")
	c.Assert(err, check.IsNil)
	defer f.Close()

	n, err := f.Seek(10, io.SeekStart)
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, int64(10))

	bytes, err = ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(len(bytes), check.Equals, 803)

	n, err = f.Seek(-50, io.SeekEnd)
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, int64(763))
	bytes, err = ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(len(bytes), check.Equals, 50)

	_, err = f.Seek(-50, io.SeekEnd)
	c.Assert(err, check.IsNil)
	n, err = f.Seek(-50, io.SeekCurrent)
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, int64(713))
	bytes, err = ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(len(bytes), check.Equals, 100)
}

func readZipArchiveAt(path string) (ResourceMap, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// file needs to stay open for http.FileSystem reads to work
	//
	// feed the binary into the zip reader and enumerate all files
	// found in the attached zip file:
	info, err := file.Stat()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return readZipArchive(file, info.Size())
}
