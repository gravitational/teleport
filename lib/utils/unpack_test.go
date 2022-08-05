/*
Copyright 2019 Gravitational, Inc.

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

package utils

import (
	"archive/tar"

	"gopkg.in/check.v1"
)

type UnpackSuite struct{}

var _ = check.Suite(&UnpackSuite{})

func (s *UnpackSuite) TestSanitizeTarPath(c *check.C) {
	cases := []struct {
		header      *tar.Header
		expectError bool
	}{
		// File path is within destination directory
		{
			header: &tar.Header{
				Name: "test1.txt",
			},
			expectError: false,
		},
		{
			header: &tar.Header{
				Name: "./dir/test2.txt",
			},
			expectError: false,
		},
		{
			header: &tar.Header{
				Name: "/dir/../dir2/test3.txt",
			},
			expectError: false,
		},
		{
			header: &tar.Header{
				Name: "./dir/test4.txt",
			},
			expectError: false,
		},
		// Linkname path is within destination directory
		{
			header: &tar.Header{
				Name:     "test5.txt",
				Linkname: "test5.txt",
			},
			expectError: false,
		},
		{
			header: &tar.Header{
				Name:     "test6.txt",
				Linkname: "./dir/test6.txt",
			},
			expectError: false,
		},
		{
			header: &tar.Header{
				Name:     "test7.txt",
				Linkname: "./dir/../dir2/test7.txt",
			},
			expectError: false,
		},
		{
			header: &tar.Header{
				Name:     "dir1/test8.txt",
				Linkname: "dir1/../dir2/test8.txt",
			},
			expectError: false,
		},
		// Name will be outside destination directory
		{
			header: &tar.Header{
				Name: "../test9.txt",
			},
			expectError: true,
		},
		{
			header: &tar.Header{
				Name: "./test/../../test10.txt",
			},
			expectError: true,
		},
		// Linkname points outside destination directory
		{
			header: &tar.Header{
				Name:     "test11.txt",
				Linkname: "../test11.txt",
			},
			expectError: true,
		},
		{
			header: &tar.Header{
				Name:     "test12.txt",
				Linkname: "./test/../../test12.txt",
			},
			expectError: true,
		},
		// Relative link that remains inside the directory
		{
			header: &tar.Header{
				Name:     "/test/dir/test13.txt",
				Linkname: "../../test2/dir2/test14.txt",
			},
			expectError: false,
		},
		// Linkname is absolute path outside extraction directory
		{
			header: &tar.Header{
				Name:     "test14.txt",
				Linkname: "/test14.txt",
			},
			expectError: true,
		},
		// Linkname is absolute path inside extraction directory
		{
			header: &tar.Header{
				Name:     "test15.txt",
				Linkname: "/tmp/test15.txt",
			},
			expectError: false,
		},
	}

	for _, tt := range cases {
		comment := check.Commentf("Name: %v LinkName: %v", tt.header.Name, tt.header.Linkname)
		err := sanitizeTarPath(tt.header, "/tmp")
		c.Assert(err != nil, check.Equals, tt.expectError, comment)
	}
}
