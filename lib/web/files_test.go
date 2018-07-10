/*
Copyright 2018 Gravitational, Inc.

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
	"gopkg.in/check.v1"
)

type FilesSuite struct {
}

var _ = check.Suite(&FilesSuite{})

func (s *FilesSuite) TestRemoteDestination(c *check.C) {
	validCases := map[string]string{
		"/relative/file.txt":       "./file.txt",
		"/relative/dir/file.txt":   "./dir/file.txt",
		"/relative/../file.txt":    "./../file.txt",
		"/relative/.":              "./.",
		"/absolute/file.txt":       "/file.txt",
		"/absolute/dir/file.txt":   "/dir/file.txt",
		"/absolute/./dir/file.txt": "/./dir/file.txt",
	}

	invalidCases := []string{
		"wrong_prefix/file.txt",
		"",
	}

	ft := fileTransfer{}
	for url, expected := range validCases {
		comment := check.Commentf(url)
		path, err := ft.resolveRemoteDest(url)
		c.Assert(err, check.IsNil, comment)
		c.Assert(path, check.Equals, expected)
	}

	for _, url := range invalidCases {
		_, err := ft.resolveRemoteDest(url)
		c.Assert(err, check.NotNil)
	}
}
