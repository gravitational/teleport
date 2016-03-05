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

// Package recorder implements session recorder - it captures all the
// output that can be later replayed in terminals
package recorder

import (
	"io"
	"time"
)

// Recorder is a session recoreder and playback
// interface
type Recorder interface {
	// GetChunkWriter returns a new writer that can record
	// chunks with active session data to the recording server
	GetChunkWriter(id string) (ChunkWriteCloser, error)
	// GetChunkReader returns a reader of recorded chunks
	GetChunkReader(id string) (ChunkReadCloser, error)
}

// Chunk is a piece of recorded session on some node
type Chunk struct {
	// Data is a captured terminal data
	Data []byte `json:"data"`
	// Delay is delay before the previous chunk appeared
	Delay time.Duration `json:"delay"`
	// ServerID is a server ID of the recorded session
	ServerID string `json:"server_id"`
}

// ChunkReader is a playback of a recorded session
type ChunkReader interface {
	// ReadChunks returns a list of chunks from start to end indexes
	ReadChunks(start int, end int) ([]Chunk, error)
	GetChunksCount() (uint64, error)
}

// ChunkReadCloser implements chunk reader + adds closer
type ChunkReadCloser interface {
	ChunkReader
	io.Closer
}

// ChunkWriter is a session recorder
type ChunkWriter interface {
	// WriteChunks stores recorded chunks in the registry
	WriteChunks([]Chunk) error
}

// ChunkWriteCloser is a chunk writer with Closer interface
type ChunkWriteCloser interface {
	ChunkWriter
	io.Closer
}

// ChunkReadWriteCloser is a chunk reader, writer and closer interfaces
type ChunkReadWriteCloser interface {
	ChunkReader
	ChunkWriter
	io.Closer
}
