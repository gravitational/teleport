package recordingencryptionv1

import (
	"context"
	"io"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"

	"github.com/gravitational/teleport/lib/events"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
)

// ServiceConfig captures everything a [Service] requires to fulfill requests.
type ServiceConfig struct {
	Logger   *slog.Logger
	Uploader events.EncryptedRecordingUploader
}

// NewService returns a new [Service] based on the given [ServiceConfig].
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Service{
		logger:   cfg.Logger.With("component", teleport.ComponentRecordingEncryption),
		uploader: cfg.Uploader,
	}, nil
}

// Service implements a gRPC server for interacting with encrypted recordings.
type Service struct {
	recordingencryptionv1.UnimplementedRecordingEncryptionServiceServer

	logger   *slog.Logger
	uploader events.EncryptedRecordingUploader
}

// UploadEncryptedRecording responds to requests to upload recordings that have already been encrypted using an
// async recording mode.
func (s *Service) UploadEncryptedRecording(stream grpc.ClientStreamingServer[recordingencryptionv1.UploadEncryptedRecordingRequest, recordingencryptionv1.UploadEncryptedRecordingResponse]) (err error) {
	ctx, cancel := context.WithCancel(stream.Context())
	defer func() {
		cancel()
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to receive encrypted recording", "error", err)
		}

		if err := trace.Wrap(stream.SendAndClose(nil)); err != nil {
			s.logger.ErrorContext(ctx, "failed to signal successful recording upload to client", "error", err)
		}
	}()

	pipe, errCh := s.uploader.UploadEncryptedRecording(ctx)
	for {
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				close(pipe)
				break
			}
			return trace.Wrap(err)
		}

		select {
		case err := <-errCh:
			return trace.Wrap(err)
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		default:
			pipe <- req
		}
	}

	return trace.Wrap(<-errCh)
}
