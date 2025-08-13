package reverseproxy

import (
	"context"
	"os"
	"sync"

	"github.com/gravitational/trace"
)

func (f *Forwarder) createTemporaryFileForRewind(ctx context.Context) (*closeOnceFile, func(), error) {
	var file *closeOnceFile
	{
		tmpFile, err := os.CreateTemp("", "teleport-reverse-proxy-")
		if err != nil {
			return nil, nil, trace.Wrap(err, "failed to create temporary file for request body rewind")
		}
		file = &closeOnceFile{File: tmpFile}
	}

	// Ensure the temporary file is removed after use.
	clean := func() {
		if err := file.Close(); err != nil {
			f.logger.ErrorContext(ctx, "Failed to close temporary file for request body rewind",
				"error", err,
			)
		}
		if err := os.Remove(file.Name()); err != nil {
			f.logger.ErrorContext(ctx, "Failed to remove temporary file for request body rewind",
				"error", err,
			)
		}
	}
	return file, clean, nil
}

// closeOnceFile wraps os.File to make Close idempotent.
type closeOnceFile struct {
	*os.File
	closeOnce sync.Once
	closeErr  error
}

// Close closes the file only once.
func (sf *closeOnceFile) Close() error {
	sf.closeOnce.Do(func() {
		sf.closeErr = sf.File.Close()
	})
	return sf.closeErr
}
