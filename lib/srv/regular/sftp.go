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

package regular

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
)

// number of goroutines that copy SFTP data from a SSH channel to
// and from anonymous pipes
const copyingGoroutines = 2

type sftpSubsys struct {
	logger *slog.Logger

	fileTransferReq *srv.FileTransferRequest
	sftpCmd         *exec.Cmd
	serverCtx       *srv.ServerContext
	errCh           chan error
}

func newSFTPSubsys(fileTransferReq *srv.FileTransferRequest) (*sftpSubsys, error) {
	return &sftpSubsys{
		logger:          slog.With(teleport.ComponentKey, teleport.ComponentSubsystemSFTP),
		fileTransferReq: fileTransferReq,
	}, nil
}

func (s *sftpSubsys) Start(ctx context.Context,
	serverConn *ssh.ServerConn,
	ch ssh.Channel, req *ssh.Request,
	serverCtx *srv.ServerContext,
) error {
	// Check that file copying is allowed Node-wide again here in case
	// this connection was proxied, the proxy doesn't know if file copying
	// is allowed for certain Nodes.
	if !serverCtx.AllowFileCopying {
		serverCtx.GetServer().EmitAuditEvent(context.WithoutCancel(ctx), &apievents.SFTP{
			Metadata: apievents.Metadata{
				Code: events.SFTPDisallowedCode,
				Type: events.SFTPEvent,
				Time: time.Now(),
			},
			UserMetadata:   serverCtx.Identity.GetUserMetadata(),
			ServerMetadata: serverCtx.GetServer().TargetMetadata(),
			Error:          srv.ErrNodeFileCopyingNotPermitted.Error(),
		})
		return srv.ErrNodeFileCopyingNotPermitted
	}

	s.serverCtx = serverCtx

	// Create two sets of anonymous pipes to give the child process
	// access to the SSH channel
	chReadPipeOut, chReadPipeIn, err := os.Pipe()
	if err != nil {
		return trace.Wrap(err)
	}
	defer chReadPipeOut.Close()
	chWritePipeOut, chWritePipeIn, err := os.Pipe()
	if err != nil {
		return trace.Wrap(err)
	}
	defer chWritePipeIn.Close()
	// Create anonymous pipe that the child will send audit information
	// over
	auditPipeOut, auditPipeIn, err := os.Pipe()
	if err != nil {
		return trace.Wrap(err)
	}
	defer auditPipeIn.Close()

	// Create child process to handle SFTP connection
	execRequest, err := srv.NewExecRequest(serverCtx, teleport.SFTPSubsystem)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := serverCtx.SetExecRequest(execRequest); err != nil {
		return trace.Wrap(err)
	}
	if err := serverCtx.SetSSHRequest(req); err != nil {
		return trace.Wrap(err)
	}

	s.sftpCmd, err = srv.ConfigureCommand(serverCtx, chReadPipeOut, chWritePipeIn, auditPipeIn)
	if err != nil {
		return trace.Wrap(err)
	}
	s.sftpCmd.Stdout = os.Stdout
	s.sftpCmd.Stderr = os.Stderr

	s.logger.DebugContext(ctx, "starting SFTP process")
	err = s.sftpCmd.Start()
	if err != nil {
		return trace.Wrap(err)
	}
	execRequest.Continue()

	// Send the file transfer request if applicable. The SFTP process
	// expects the file transfer request data will end with a null byte,
	// so if there is no request to send just send a null byte so the
	// SFTP process can detect that no request was sent.
	encodedReq := []byte{0x0}
	if s.fileTransferReq != nil {
		encodedReq, err = json.Marshal(s.fileTransferReq)
		if err != nil {
			return trace.Wrap(err)
		}
		encodedReq = append(encodedReq, 0x0)
	}
	_, err = chReadPipeIn.Write(encodedReq)
	if err != nil {
		return trace.Wrap(err)
	}

	// Copy the SSH channel to and from the anonymous pipes
	s.errCh = make(chan error, copyingGoroutines)
	go func() {
		defer chReadPipeIn.Close()

		_, err := io.Copy(chReadPipeIn, ch)
		s.errCh <- err
	}()
	go func() {
		defer chWritePipeOut.Close()

		_, err := io.Copy(ch, chWritePipeOut)
		s.errCh <- err
	}()

	// Read and emit audit events from the child process
	go func() {
		defer auditPipeOut.Close()

		// Create common fields for events
		serverMeta := serverCtx.GetServer().TargetMetadata()
		sessionMeta := serverCtx.GetSessionMetadata()
		userMeta := serverCtx.Identity.GetUserMetadata()
		connectionMeta := apievents.ConnectionMetadata{
			RemoteAddr: serverConn.RemoteAddr().String(),
			LocalAddr:  serverConn.LocalAddr().String(),
		}

		r := bufio.NewReader(auditPipeOut)
		for {
			// Read up to a NULL byte, the child process uses this to
			// delimit audit events
			eventStr, err := r.ReadString(0x0)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					s.logger.WarnContext(ctx, "Failed to read SFTP event", "error", err)
				}
				return
			}

			var oneOfEvent apievents.OneOf
			err = (&jsonpb.Unmarshaler{}).Unmarshal(strings.NewReader(eventStr[:len(eventStr)-1]), &oneOfEvent)
			if err != nil {
				s.logger.WarnContext(ctx, "Failed to unmarshal SFTP event", "error", err)
				continue
			}
			event, err := apievents.FromOneOf(oneOfEvent)
			if err != nil {
				s.logger.WarnContext(ctx, "Failed to convert SFTP event from OneOf", "error", err)
				continue
			}

			event.SetClusterName(serverCtx.ClusterName)
			switch e := event.(type) {
			case *apievents.SFTP:
				e.ServerMetadata = serverMeta
				e.SessionMetadata = sessionMeta
				e.UserMetadata = userMeta
				e.ConnectionMetadata = connectionMeta
			case *apievents.SFTPSummary:
				e.ServerMetadata = serverMeta
				e.SessionMetadata = sessionMeta
				e.UserMetadata = userMeta
				e.ConnectionMetadata = connectionMeta
			default:
				s.logger.WarnContext(ctx, "Unknown event type received from SFTP server process", "error", err, "event_type", event.GetType())
			}

			if err := serverCtx.GetServer().EmitAuditEvent(ctx, event); err != nil {
				s.logger.WarnContext(ctx, "Failed to emit SFTP event", "error", err)
			}
		}
	}()

	return nil
}

func (s *sftpSubsys) Wait() error {
	ctx := context.Background()
	waitErr := s.sftpCmd.Wait()
	s.logger.DebugContext(ctx, "SFTP process finished")

	s.serverCtx.SendExecResult(ctx, srv.ExecResult{
		Command: s.sftpCmd.String(),
		Code:    s.sftpCmd.ProcessState.ExitCode(),
	})

	errs := []error{waitErr}
	for range copyingGoroutines {
		err := <-s.errCh
		if err != nil && !utils.IsOKNetworkError(err) {
			s.logger.WarnContext(ctx, "Connection problem", "error", err)
			errs = append(errs, err)
		}
	}

	return trace.NewAggregate(errs...)
}
