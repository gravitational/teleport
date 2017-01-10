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
	"net/http"
	"os"

	"github.com/gravitational/teleport"

	"gopkg.in/check.v1"
)

type StaticSuite struct {
}

var _ = check.Suite(&StaticSuite{})

func (s *StaticSuite) SetUpSuite(c *check.C) {
	debugAssetsPath = "../../web/dist"
}

func (s *StaticSuite) TestDebugModeEnv(c *check.C) {
	c.Assert(isDebugMode(), check.Equals, false)
	os.Setenv(teleport.DebugEnvVar, "no")
	c.Assert(isDebugMode(), check.Equals, false)
	os.Setenv(teleport.DebugEnvVar, "0")
	c.Assert(isDebugMode(), check.Equals, false)
	os.Setenv(teleport.DebugEnvVar, "1")
	c.Assert(isDebugMode(), check.Equals, true)
	os.Setenv(teleport.DebugEnvVar, "true")
	c.Assert(isDebugMode(), check.Equals, true)
}

func (s *StaticSuite) TestLocalFS(c *check.C) {
	fs, err := NewStaticFileSystem(true)
	c.Assert(err, check.IsNil)
	c.Assert(fs, check.NotNil)

	checkFS(fs, c)
}

func (s *StaticSuite) TestZipFS(c *check.C) {
	fs, err := readZipArchive("../../fixtures/assets.zip")
	c.Assert(err, check.IsNil)
	c.Assert(fs, check.NotNil)

	checkFS(fs, c)
}

func checkFS(fs http.FileSystem, c *check.C) {
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
	bytes, err = ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(len(bytes), check.Equals, 50)

	f.Seek(-50, io.SeekEnd)
	n, err = f.Seek(-50, io.SeekCurrent)
	c.Assert(err, check.IsNil)
	bytes, err = ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(len(bytes), check.Equals, 100)
}
