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

package identityfile

import (
	"io/fs"
	"os"
	"sync"
	"time"
)

type InMemoryFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
	content []byte
}

// Name returns the file's name
func (fi InMemoryFileInfo) Name() string {
	return fi.name
}

// Size returns the file size (calculated when writing the file)
func (fi InMemoryFileInfo) Size() int64 {
	return fi.size
}

// Mode returns the fs.FileMode
func (fi InMemoryFileInfo) Mode() fs.FileMode {
	return fi.mode
}

// ModTime returns the last modification time
func (fi InMemoryFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir checks whether the file is a directory
func (fi InMemoryFileInfo) IsDir() bool {
	return fi.isDir
}

// Sys is platform independent
// InMemoryFileInfo's implementation is no-op
func (fi InMemoryFileInfo) Sys() interface{} {
	return nil
}

func NewInMemoryConfigWriter() InMemoryConfigWriter {
	return InMemoryConfigWriter{
		mux:   &sync.RWMutex{},
		files: make(map[string]InMemoryFileInfo),
	}
}

// InMemoryConfigWriter is a basic virtual file system abstraction that writes into memory
//  instead of writing to a more persistent storage.
type InMemoryConfigWriter struct {
	mux   *sync.RWMutex
	files map[string]InMemoryFileInfo
}

// WriteFile writes the given data to path `name`
// It replaces the file if it already exists
func (m InMemoryConfigWriter) WriteFile(name string, data []byte, perm os.FileMode) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.files[name] = InMemoryFileInfo{
		name:    name,
		size:    int64(len(data)),
		mode:    perm,
		modTime: time.Now(),
		content: data,
		isDir:   false,
	}

	return nil
}

// Remove the file.
// If the file does not exist, Remove is a no-op
func (m InMemoryConfigWriter) Remove(name string) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	delete(m.files, name)
	return nil
}

// Stat returns the FileInfo of the given file.
// Returns fs.ErrNotExists if the file is not present
func (m InMemoryConfigWriter) Stat(name string) (fs.FileInfo, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	f, found := m.files[name]
	if !found {
		return nil, fs.ErrNotExist
	}

	return f, nil
}

// Read returns the file contents.
// Returns fs.ErrNotExists if the file is not present
func (m InMemoryConfigWriter) Read(name string) ([]byte, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	f, found := m.files[name]
	if !found {
		return nil, fs.ErrNotExist
	}

	return f.content, nil
}
