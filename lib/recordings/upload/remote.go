package upload

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/recordings"
	"github.com/gravitational/teleport/lib/session"
)

type RemoteUpload struct {
	id        string
	sessionID session.ID
	stream    grpc.ClientStreamingClient[recordingencryptionv1.UploadRecordingRequest, recordingencryptionv1.UploadRecordingResponse]
}

type RemoteUploader struct {
	client *authclient.Client
}

func NewRemoteUploader(client *authclient.Client) *RemoteUploader {
	return &RemoteUploader{client}
}

func (r *RemoteUploader) CreateEncryptedUpload(ctx context.Context, sessionID session.ID) (recordings.EncryptedUpload, error) {
	stream, err := r.client.UploadRecording(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &RemoteUpload{
		stream:    stream,
		sessionID: sessionID,
	}, nil

}

func (u *RemoteUpload) GetID() string {
	return u.id
}

func (u *RemoteUpload) UploadPart(ctx context.Context, part []byte) error {
	if err := u.stream.Send(&recordingencryptionv1.UploadRecordingRequest{
		Upload: &recordingencryptionv1.Upload{
			SessionId: u.sessionID.String(),
		},
		Part: part,
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (u *RemoteUpload) Complete(ctx context.Context) error {
	_, err := u.stream.CloseAndRecv()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (u *RemoteUpload) Close(ctx context.Context) error {
	err := u.stream.CloseSend()
	return trace.Wrap(err)
}
