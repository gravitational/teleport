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
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv"
	reexecutils "github.com/gravitational/teleport/lib/sshutils/reexec"
	sftputils "github.com/gravitational/teleport/lib/sshutils/sftp"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/session/reexec"
	"github.com/gravitational/teleport/session/reexec/reexecconstants"
	"github.com/gravitational/teleport/session/reexec/reexecsftp"
	sessionsftputils "github.com/gravitational/teleport/session/sftputils"
)

type sftpSubsys struct {
	logger *slog.Logger

	fileTransferReq *reexecsftp.FileTransferRequest
	sftpCmd         *reexec.CommandExecutor
	serverCtx       *srv.ServerContext

	// waitForOutputStreams tracks goroutines that copy stderr/stdout from child
	// reexec and shell processes. This is necessary due to the use of custom pipes,
	// which exec.Cmd does not wait for closure of in cmd.Wait().
	waitForOutputStreams sync.WaitGroup
}

func newSFTPSubsys(fileTransferReq *reexecsftp.FileTransferRequest) (*sftpSubsys, error) {
	return &sftpSubsys{
		logger:          slog.With(teleport.ComponentKey, "subsystem:sftp"),
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
			ServerMetadata: serverCtx.GetServer().EventMetadata(),
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
	execRequest, err := srv.NewExecRequest(serverCtx, reexecconstants.SFTPSubCommand)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := serverCtx.SetExecRequest(execRequest); err != nil {
		return trace.Wrap(err)
	}
	if err := serverCtx.SetSSHRequest(req); err != nil {
		return trace.Wrap(err)
	}
	s.sftpCmd, err = serverCtx.ConfigureCommand(map[reexec.FileFD]*os.File{
		reexec.StdinFile:  chReadPipeOut,
		reexec.StdoutFile: chWritePipeIn,
		reexec.StderrFile: auditPipeIn,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Capture stderr.
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		return trace.Wrap(err)
	}
	defer stderrW.Close()
	s.sftpCmd.Stderr = stderrW

	s.waitForOutputStreams.Go(func() {
		defer stderrR.Close()

		childErr, err := reexecutils.ReadChildErrorWithContext(stderrR, &reexecutils.ErrorContext{
			DecisionContext: s.serverCtx.Identity.AccessPermit.DecisionContext,
			Login:           s.serverCtx.Identity.Login,
		})
		if err != nil {
			s.logger.WarnContext(context.WithoutCancel(ctx), "Failed to read child process stderr", "error", err)
			return
		}
		if childErr == "" {
			return
		}

		if _, err := io.WriteString(ch.Stderr(), childErr); err != nil {
			s.logger.WarnContext(context.WithoutCancel(ctx), "Failed to propagate child process stderr to client", "error", err)
		}
	})

	s.logger.DebugContext(ctx, "starting SFTP process")
	err = s.sftpCmd.Start()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.sftpCmd.Continue(); err != nil {
		return trace.Wrap(err)
	}

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

	// Copy the SSH channel to and from the anonymous pipes. The input copy from
	// the SSH channel must not gate Wait(), or early child-process failures can
	// deadlock waiting for the client to close the channel before we send the
	// exit status.
	go func() {
		defer chReadPipeIn.Close()
		if _, err := io.Copy(chReadPipeIn, ch); err != nil && !utils.IsOKNetworkError(err) {
			s.logger.WarnContext(ctx, "Failure reading from SFTP subsystem", "error", err)
		}
	}()
	s.waitForOutputStreams.Go(func() {
		defer chWritePipeOut.Close()
		if _, err := io.Copy(ch, chWritePipeOut); err != nil && !utils.IsOKNetworkError(err) {
			s.logger.WarnContext(ctx, "Failure writing to SFTP subsystem", "error", err)
		}
	})

	// Read and emit audit events from the child process
	go func() {
		defer auditPipeOut.Close()

		// Create common fields for events
		serverMeta := serverCtx.GetServer().EventMetadata()
		sessionMeta := serverCtx.GetSessionMetadata()
		userMeta := serverCtx.Identity.GetUserMetadata()
		connectionMeta := apievents.ConnectionMetadata{
			RemoteAddr: serverConn.RemoteAddr().String(),
			LocalAddr:  serverConn.LocalAddr().String(),
		}

		dec := json.NewDecoder(auditPipeOut)
		for {
			var ev sessionsftputils.Event
			if err := dec.Decode(&ev); err != nil {
				if !errors.Is(err, io.EOF) {
					s.logger.WarnContext(ctx, "Failed to read SFTP event", "error", err)
				}
				return
			}

			var event apievents.AuditEvent
			if ev.SFTP != nil {
				e, err := sftputils.SFTPEventToProto(ev.SFTP)
				if err != nil {
					s.logger.WarnContext(ctx, "Failed to convert SFTP event", "error", err)
					continue
				}
				e.SetClusterName(serverCtx.ClusterName)
				e.ServerMetadata = serverMeta
				e.SessionMetadata = sessionMeta
				e.UserMetadata = userMeta
				e.ConnectionMetadata = connectionMeta
				event = e
			} else if ev.Summary != nil {
				e := sftputils.SFTPSummaryEventToProto(ev.Summary)
				e.SetClusterName(serverCtx.ClusterName)
				e.ServerMetadata = serverMeta
				e.SessionMetadata = sessionMeta
				e.UserMetadata = userMeta
				e.ConnectionMetadata = connectionMeta
				event = e
			} else {
				s.logger.WarnContext(ctx, "Unknown event type received from SFTP server process")
				continue
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
	s.waitForOutputStreams.Wait()
	s.logger.DebugContext(ctx, "SFTP process finished")

	s.serverCtx.SendExecResult(ctx, srv.ExecResult{
		Command: s.sftpCmd.String(),
		Code:    s.sftpCmd.ProcessState.ExitCode(),
	})

	return trace.Wrap(waitErr)
}
