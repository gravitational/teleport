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

package utils

import (
	"io"
	"os"
)

// IsFile returns true if a given file path points to an existing file
func IsFile(fp string) bool {
	fi, err := os.Stat(fp)
	if err == nil {
		return !fi.IsDir()
	}
	return false
}

// IsDir is a helper function to quickly check if a given path is a valid directory
func IsDir(dirPath string) bool {
	fi, err := os.Stat(dirPath)
	if err == nil {
		return fi.IsDir()
	}
	return false
}

// ReadAll is similarl to ioutil.ReadAll, except it doesn't use ever-increasing
// internal buffer, instead asking for the exact buffer size.
//
// This is useful when you want to limit the sze of Read/Writes (websockets)
func ReadAll(r io.Reader, bufsize int) (out []byte, err error) {
	buff := make([]byte, bufsize)
	n := 0
	for err == nil {
		n, err = r.Read(buff)
		if n > 0 {
			out = append(out, buff[:n]...)
		}
	}
	if err == io.EOF {
		err = nil
	}
	return out, err
}
