package summarizer

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

// sessionReader reads session data from a channel of audit events. It supports
// reading SSH, Kubernetes and database session data.
type sessionReader struct {
	ctx      context.Context
	eventsCh chan apievents.AuditEvent
	errCh    chan error
	buf      bytes.Buffer
	cancel   context.CancelFunc
}

func newSessionReader(
	ctx context.Context, streamer events.SessionStreamer, sessionID session.ID,
) *sessionReader {
	// Create a derived context that will be canceled when this reader is closed,
	// closing the underlying event stream.
	ctx, cancel := context.WithCancel(ctx)
	eventsCh, errCh := streamer.StreamSessionEvents(ctx, sessionID, 0)
	return &sessionReader{
		ctx:      ctx,
		eventsCh: eventsCh,
		errCh:    errCh,
		cancel:   cancel,
	}
}

// Read reads up to n bytes from the session data. It blocks until either n
// bytes are read, or there are no more events to read and EOF is reached.
func (r *sessionReader) Read(out []byte) (int, error) {
	total, err := r.buf.Read(out)
	// EOF on the buffer doesn't mean we have run out of data.
	if err != nil && !errors.Is(err, io.EOF) {
		return total, err
	}

	for total < len(out) {
		select {
		case event, ok := <-r.eventsCh:
			if !ok {
				// End of data.
				return total, io.EOF
			}
			switch e := event.(type) {
			case *apievents.SessionPrint:
				total += r.consumeSlice(out[total:], e.Data)
			case *apievents.DatabaseSessionQuery:
				total += r.consumeSlice(out[total:], []byte(e.DatabaseQuery))
			}
		case err := <-r.errCh:
			return total, trace.Wrap(err)
		case <-r.ctx.Done():
			return total, trace.Wrap(r.ctx.Err())
		}
	}

	return total, nil
}

// consumeSlice copies bytes from dest to src and stores remaining bytes from
// src in the buffer.
func (r *sessionReader) consumeSlice(dest []byte, src []byte) int {
	n := copy(dest, src)
	if n < len(src) {
		r.buf.Write(src[n:])
	}
	return n
}

func (r *sessionReader) Close() error {
	r.cancel()
	return nil
}
