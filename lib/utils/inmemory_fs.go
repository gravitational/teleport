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
func (fi *InMemoryFile) Sys() any {
	return nil
}

// Content returns the file bytes
func (fi *InMemoryFile) Content() []byte {
	return fi.content
}
