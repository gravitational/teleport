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

package identityfile

import (
	"io/fs"
	"os"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/utils"
)

type InMemoryFS map[string]*utils.InMemoryFile

type InMemoryConfigWriterOption func(*InMemoryConfigWriter)

func WithClock(clock clockwork.Clock) InMemoryConfigWriterOption {
	return func(w *InMemoryConfigWriter) {
		w.clock = clock
	}
}

// NewInMemoryConfigWriter creates a new virtual file system
// It stores the files contents and their properties in memory
func NewInMemoryConfigWriter(options ...InMemoryConfigWriterOption) *InMemoryConfigWriter {
	w := &InMemoryConfigWriter{
		mux:   &sync.RWMutex{},
		clock: clockwork.NewRealClock(),
		files: InMemoryFS{},
	}
	for _, option := range options {
		option(w)
	}
	return w
}

// InMemoryConfigWriter is a basic virtual file system abstraction that writes into memory
//
//	instead of writing to a more persistent storage.
type InMemoryConfigWriter struct {
	mux   *sync.RWMutex
	clock clockwork.Clock
	files InMemoryFS
}

// WriteFile writes the given data to path `name`
// It replaces the file if it already exists
func (m *InMemoryConfigWriter) WriteFile(name string, data []byte, perm os.FileMode) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.files[name] = utils.NewInMemoryFile(name, perm, m.clock.Now(), data)

	return nil
}

// Remove the file.
// If the file does not exist, Remove is a no-op
func (m *InMemoryConfigWriter) Remove(name string) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	delete(m.files, name)
	return nil
}

// Stat returns the FileInfo of the given file.
// Returns fs.ErrNotExists if the file is not present
func (m *InMemoryConfigWriter) Stat(name string) (fs.FileInfo, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	f, found := m.files[name]
	if !found {
		return nil, fs.ErrNotExist
	}

	return f, nil
}

// ReadFile returns the file contents.
// Returns fs.ErrNotExists if the file is not present
func (m *InMemoryConfigWriter) ReadFile(name string) ([]byte, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	f, found := m.files[name]
	if !found {
		return nil, fs.ErrNotExist
	}

	return f.Content(), nil
}

// Open is not implemented but exists here to satisfy the io/fs. interface.
func (m *InMemoryConfigWriter) Open(name string) (fs.File, error) {
	return nil, trace.NotImplemented("Open is not implemented for InMemoryConfigWriter")
}

func (m *InMemoryConfigWriter) WithReadonlyFiles(fn func(InMemoryFS) error) error {
	m.mux.RLock()
	defer m.mux.RUnlock()
	return fn(m.files)
}
