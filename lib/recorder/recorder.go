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
package recorder

import (
	"io"
	"time"
)

type Recorder interface {
	GetChunkWriter(id string) (ChunkWriteCloser, error)
	GetChunkReader(id string) (ChunkReadCloser, error)
}

type Chunk struct {
	Data  []byte        `json:"data"`  // captured terminal data
	Delay time.Duration `json:"delay"` // delay before the previous chunk
}

type ChunkReader interface {
	ReadChunks(start int, end int) ([]Chunk, error)
}

type ChunkReadCloser interface {
	ChunkReader
	io.Closer
}

type ChunkWriter interface {
	WriteChunks([]Chunk) error
}

type ChunkWriteCloser interface {
	ChunkWriter
	io.Closer
}

type ChunkReadWriteCloser interface {
	ChunkReader
	ChunkWriter
	io.Closer
}
