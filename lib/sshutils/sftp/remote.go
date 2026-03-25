/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package sftp

import (
	"context"
	"io"
	"os"
	portablepath "path"
	"strings"

	"github.com/gravitational/trace"
	"github.com/pkg/sftp"

	"github.com/gravitational/teleport"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
)

// RemoteFS provides API for accessing the files on
// the local file system
type RemoteFS struct {
	*sftp.Client
	session io.Closer
}

// NewRemoteFilesystem creates a new FileSystem over SFTP.
func NewRemoteFilesystem(c *sftp.Client) *RemoteFS {
	return &RemoteFS{Client: c}
}

// OpenRemoteFilesystem opens a new remote file system on the given ssh client.
func OpenRemoteFilesystem(ctx context.Context, sshClient *tracessh.Client, moderatedSessionID string) (fs *RemoteFS, openErr error) {
	s, err := sshClient.NewSessionWithParams(ctx, &tracessh.SessionParams{
		ModeratedSessionID: moderatedSessionID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if openErr != nil {
			s.Close()
		}
	}()

	// File transfers in a moderated session require this variable
	// to check for approval on the ssh server
	// TODO(Joerger): DELETE IN v20.0.0 - moderated session ID is provided
	// in the session channel params above instead of indirectly through env vars.
	if moderatedSessionID != "" {
		s.Setenv(ctx, EnvModeratedSessionID, moderatedSessionID)
	}

	pe, err := s.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.RequestSubsystem(ctx, teleport.SFTPSubsystem); err != nil {
		// If the subsystem request failed and a generic error is
		// returned, return the session's stderr as the error if it's
		// non-empty, as the session's stderr may have a more useful
		// error message. String comparison is only used here because
		// the error is not exported.
		if strings.Contains(err.Error(), "ssh: subsystem request failed") {
			var sb strings.Builder
			if n, _ := io.Copy(&sb, pe); n > 0 {
				return nil, trace.Errorf("%s", sb.String())
			}
		}
		return nil, trace.Wrap(err)
	}
	pw, err := s.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pr, err := s.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sftpClient, err := sftp.NewClientPipe(pr, pw,
		// Use concurrent stream to speed up transfer on slow networks as described in
		// https://github.com/gravitational/teleport/issues/20579
		sftp.UseConcurrentReads(true),
		sftp.UseConcurrentWrites(true),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &RemoteFS{
		Client:  sftpClient,
		session: s,
	}, nil
}

func (r *RemoteFS) Type() string {
	return "remote"
}

func (r *RemoteFS) ReadDir(path string) ([]os.FileInfo, error) {
	fileInfos, err := r.Client.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for i := range fileInfos {
		// If the file is a valid symlink, return the info of the linked file.
		if fileInfos[i].Mode()&os.ModeSymlink != 0 {
			resolvedInfo, err := r.Stat(portablepath.Join(path, fileInfos[i].Name()))
			if err == nil {
				fileInfos[i] = resolvedInfo
			}
		}
	}

	return fileInfos, nil
}

func (r *RemoteFS) Open(path string) (File, error) {
	return r.OpenFile(path, os.O_RDONLY)
}

func (r *RemoteFS) Create(path string, _ int64) (File, error) {
	return r.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
}

func (r *RemoteFS) OpenFile(path string, flags int) (File, error) {
	return r.Client.OpenFile(path, flags)
}

func (r *RemoteFS) Mkdir(path string) error {
	return r.Client.MkdirAll(path)
}

func (r *RemoteFS) Readlink(name string) (string, error) {
	return r.Client.ReadLink(name)
}

func (r *RemoteFS) Close() error {
	var sessionErr error
	if r.session != nil {
		sessionErr = r.session.Close()
	}
	return trace.NewAggregate(sessionErr, r.Client.Close())
}
