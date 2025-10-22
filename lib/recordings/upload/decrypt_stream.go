package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/recordings"
	"github.com/gravitational/teleport/lib/session"
)

type DecryptingStream struct {
	sessionID session.ID
	decrypter events.DecryptionWrapper
	stream    apievents.Stream
	status    apievents.StreamStatus
	m         sync.Mutex
	cancel    context.CancelFunc
}

type DecryptingStreamUploader struct {
	streamer  events.Streamer
	decrypter events.DecryptionWrapper
}

func NewDecryptingStreamUploader(decrypter events.DecryptionWrapper, streamer events.Streamer) *DecryptingStreamUploader {
	return &DecryptingStreamUploader{
		decrypter: decrypter,
		streamer:  streamer,
	}
}

func (s *DecryptingStreamUploader) CreateEncryptedUpload(ctx context.Context, sessionID session.ID) (recordings.EncryptedUpload, error) {
	stream, err := s.streamer.CreateAuditStream(ctx, sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	upload := &DecryptingStream{
		sessionID: sessionID,
		stream:    stream,
		decrypter: s.decrypter,
		cancel:    cancel,
	}

	go func() {
		for {
			select {
			case status := <-stream.Status():
				upload.m.Lock()
				upload.status = status
				upload.m.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	return upload, nil
}

func (s *DecryptingStream) GetID() string {
	return s.status.UploadID
}

func (r *DecryptingStream) UploadPart(ctx context.Context, part []byte) error {
	reader := events.NewProtoReader(bytes.NewReader(part), r.decrypter)
	for {
		ev, err := reader.Read(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return trace.Wrap(err, "reading event")
		}

		if err := r.stream.RecordEvent(ctx, preparedEvent{ev}); err != nil {
			return trace.Wrap(err, "recording event")
		}
	}

	return nil
}

func (s *DecryptingStream) Complete(ctx context.Context) error {
	return trace.Wrap(s.stream.Complete(ctx))
}

func (s *DecryptingStream) Close(ctx context.Context) error {
	return trace.Wrap(s.stream.Close(ctx))
}

type preparedEvent struct {
	ev apievents.AuditEvent
}

func (p preparedEvent) GetAuditEvent() apievents.AuditEvent {
	return p.ev
}
