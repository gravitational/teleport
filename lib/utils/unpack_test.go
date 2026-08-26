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

package utils

import (
	"archive/tar"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeTarPath(t *testing.T) {
	t.Parallel()

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
		comment := fmt.Sprintf("Name: %v LinkName: %v", tt.header.Name, tt.header.Linkname)
		err := sanitizeTarPath(tt.header, "/tmp")
		require.Equal(t, err != nil, tt.expectError, comment)
	}
}
