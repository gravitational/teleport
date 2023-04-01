/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package regular

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
)

// number of goroutines that copy SFTP data from a SSH channel to
// and from anonymous pipes
const copyingGoroutines = 2

type sftpSubsys struct {
	sftpCmd   *exec.Cmd
	serverCtx *srv.ServerContext
	errCh     chan error
	log       *logrus.Entry
}

func newSFTPSubsys() (*sftpSubsys, error) {
	// TODO: add prometheus collectors?
	return &sftpSubsys{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentSubsystemSFTP,
		}),
	}, nil
}

func (s *sftpSubsys) Start(ctx context.Context, serverConn *ssh.ServerConn, ch ssh.Channel, _ *ssh.Request,
	serverCtx *srv.ServerContext,
) error {
	// Check that file copying is allowed Node-wide again here in case
	// this connection was proxied, the proxy doesn't know if file copying
	// is allowed for certain Nodes.
	if !serverCtx.AllowFileCopying {
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
	executable, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	execRequest, err := srv.NewExecRequest(serverCtx, executable+" sftp")
	if err != nil {
		return trace.Wrap(err)
	}
	if err := serverCtx.SetExecRequest(execRequest); err != nil {
		return trace.Wrap(err)
	}

	s.sftpCmd, err = srv.ConfigureCommand(serverCtx, chReadPipeOut, chWritePipeIn, auditPipeIn)
	if err != nil {
		return trace.Wrap(err)
	}
	s.sftpCmd.Stdout = os.Stdout
	s.sftpCmd.Stderr = os.Stderr

	s.log.Debug("starting SFTP process")
	err = s.sftpCmd.Start()
	if err != nil {
		return trace.Wrap(err)
	}
	// TODO: put in cgroup?
	execRequest.Continue()

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
		serverMeta := apievents.ServerMetadata{
			ServerID:        serverCtx.GetServer().HostUUID(),
			ServerHostname:  serverCtx.GetServer().GetInfo().GetHostname(),
			ServerNamespace: serverCtx.GetServer().GetNamespace(),
		}
		sessionMeta := apievents.SessionMetadata{
			SessionID: string(serverCtx.SessionID()),
			WithMFA:   serverCtx.Identity.Certificate.Extensions[teleport.CertExtensionMFAVerified],
		}
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
					s.log.WithError(err).Warn("Failed to read SFTP event.")
				}
				return
			}

			var sftpEvent apievents.SFTP
			err = jsonpb.UnmarshalString(eventStr[:len(eventStr)-1], &sftpEvent)
			if err != nil {
				s.log.WithError(err).Warn("Failed to unmarshal SFTP event.")
				continue
			}

			sftpEvent.Metadata.ClusterName = serverCtx.ClusterName
			sftpEvent.ServerMetadata = serverMeta
			sftpEvent.SessionMetadata = sessionMeta
			sftpEvent.UserMetadata = userMeta
			sftpEvent.ConnectionMetadata = connectionMeta

			if err := serverCtx.GetServer().EmitAuditEvent(ctx, &sftpEvent); err != nil {
				log.WithError(err).Warn("Failed to emit SFTP event.")
			}
		}
	}()

	return nil
}

func (s *sftpSubsys) Wait() error {
	waitErr := s.sftpCmd.Wait()
	s.log.Debug("SFTP process finished")

	s.serverCtx.SendExecResult(srv.ExecResult{
		Command: s.sftpCmd.String(),
		Code:    s.sftpCmd.ProcessState.ExitCode(),
	})

	errs := []error{waitErr}
	for i := 0; i < copyingGoroutines; i++ {
		err := <-s.errCh
		if err != nil && !utils.IsOKNetworkError(err) {
			s.log.WithError(err).Warn("Connection problem.")
			errs = append(errs, err)
		}
	}

	return trace.NewAggregate(errs...)
}
