package filesessions

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	reservationFilePerm = 0600
	combinedFilePerm    = reservationFilePerm
)

// FileOps captures file operations done by filesessions
type FileOps interface {
	CreateReservation(ctx context.Context, name string, size int64) error
	WriteReservation(ctx context.Context, name string, data io.Reader) error
	CombineParts(ctx context.Context, dst io.Writer, parts []string) error
}

func loggingClose(ctx context.Context, closer io.Closer, log *slog.Logger, msg string, args ...any) {
	if err := closer.Close(); err != nil {
		log.ErrorContext(ctx, msg, append(args, "error", err)...)
	}
}

type plainFileOps struct {
	log      *slog.Logger
	openFile utils.OpenFileWithFlagsFunc
}

var _ FileOps = &plainFileOps{}

// NewPlainFileOps returns a plaintext implementation of the FileOps interface.
func NewPlainFileOps(log *slog.Logger, openFile utils.OpenFileWithFlagsFunc) *plainFileOps {
	return &plainFileOps{log: log, openFile: openFile}
}

// CreateReservation creates the initial reservation file for
func (p *plainFileOps) CreateReservation(ctx context.Context, name string, size int64) (err error) {
	defer func() {
		if err != nil {
			err = trace.ConvertSystemError(err)
		}
	}()

	f, err := p.openFile(name, os.O_WRONLY|os.O_CREATE, reservationFilePerm)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := f.Truncate(size); err != nil {
		loggingClose(ctx, f, p.log, "failed to close file after Truncate error", "name", name)
	}

	return trace.Wrap(f.Close())
}

func (p *plainFileOps) WriteReservation(ctx context.Context, name string, data io.Reader) (err error) {
	defer func() {
		if err != nil {
			err = trace.ConvertSystemError(err)
		}
	}()

	log := p.log.With("name", name)
	// O_CREATE kepr for backwards compatibility only
	const flag = os.O_WRONLY | os.O_CREATE

	f, err := p.openFile(name, flag, reservationFilePerm)
	if err != nil {
		return trace.Wrap(err)
	}
	defer loggingClose(ctx, f, log, "failed to close reservation file during write")

	n, err := io.Copy(f, data)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := f.Truncate(n); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (p *plainFileOps) CombineParts(ctx context.Context, dst io.Writer, parts []string) error {
	for _, part := range parts {
		log := p.log.With("name", part)
		partFile, err := p.openFile(part, os.O_RDONLY, 0)
		if err != nil {
			return trace.Wrap(err)
		}
		defer loggingClose(ctx, partFile, log, "failed to close part")

		if _, err = io.Copy(dst, partFile); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
