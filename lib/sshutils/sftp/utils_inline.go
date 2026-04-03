package sftp

import (
	"github.com/pkg/sftp"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/session/sftputils"
)

//go:fix inline
type TrackedFile = sftputils.TrackedFile

//go:fix inline
func ParseFlags(req *sftp.Request) int {
	return sftputils.ParseFlags(req)
}

//go:fix inline
func ParseSFTPEvent(req *sftp.Request, workingDirectory string, reqErr error) (*apievents.SFTP, error) {
	return sftputils.ParseSFTPEvent(req, workingDirectory, reqErr)
}
