package recordings

import (
	"context"

	"github.com/gravitational/teleport/lib/session"
)

type EncryptedUploader interface {
	CreateEncryptedUpload(context.Context, session.ID) (EncryptedUpload, error)
}

type EncryptedUpload interface {
	UploadPart(context.Context, []byte) error
	Complete(context.Context) error
}
