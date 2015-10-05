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
