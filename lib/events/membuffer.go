package events

import (
	"sync"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
)

// MemBuffer is a in-memory byte buffer that implements both io.Writer and
// io.WriterAt interfaces.
type MemBuffer struct {
	buf manager.WriteAtBuffer
	// mu is a mutex to protect concurrent writes to the buffer. Even though the
	// underlying buffer is thread-safe, we need to prevent a race condition in
	// [MemBuffer.Write] between checking the length of the buffer and writing to
	// it.
	mu sync.Mutex
}

func (b *MemBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.WriteAt(p, int64(len(b.buf.Bytes())))
}

func (b *MemBuffer) WriteAt(p []byte, pos int64) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.WriteAt(p, pos)
}

// Bytes return the underlying byte slice.
func (b *MemBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Bytes()
}
