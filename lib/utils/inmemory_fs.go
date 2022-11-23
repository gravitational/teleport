/*
Copyright 2022 Gravitational, Inc.

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
	"io/fs"
	"time"
)

// InMemoryFile stores the required properties to emulate a File in memory
// It contains the File properties like name, size, mode
// It also contains the File contents
// It does not support folders
type InMemoryFile struct {
	name    string
	mode    fs.FileMode
	modTime time.Time
	content []byte
}

func NewInMemoryFile(name string, mode fs.FileMode, modTime time.Time, content []byte) *InMemoryFile {
	return &InMemoryFile{
		name:    name,
		mode:    mode,
		modTime: modTime,
		content: content,
	}
}

// Name returns the file's name
func (fi *InMemoryFile) Name() string {
	return fi.name
}

// Size returns the file size (calculated when writing the file)
func (fi *InMemoryFile) Size() int64 {
	return int64(len(fi.content))
}

// Mode returns the fs.FileMode
func (fi *InMemoryFile) Mode() fs.FileMode {
	return fi.mode
}

// ModTime returns the last modification time
func (fi *InMemoryFile) ModTime() time.Time {
	return fi.modTime
}

// IsDir checks whether the file is a directory
func (fi *InMemoryFile) IsDir() bool {
	return false
}

// Sys is platform independent
// InMemoryFile's implementation is no-op
func (fi *InMemoryFile) Sys() interface{} {
	return nil
}

// Content returns the file bytes
func (fi *InMemoryFile) Content() []byte {
	return fi.content
}
