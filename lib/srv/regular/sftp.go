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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type sftpSubsys struct {
	sftpCmd *exec.Cmd
	ch      ssh.Channel
	errCh   chan error
	log     *logrus.Entry
}

func newSFTPSubsys() (*sftpSubsys, error) {
	// TODO: add prometheus collectors?
	return &sftpSubsys{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentSubsystemSFTP,
		}),
	}, nil
}

func (s *sftpSubsys) Start(ctx context.Context, serverConn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, serverCtx *srv.ServerContext) error {
	s.ch = ch

	err := req.Reply(true, nil)
	if err != nil {
		return trace.Wrap(err)
	}

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
	serverCtx.ExecRequest, err = srv.NewExecRequest(serverCtx, executable+" sftp")
	if err != nil {
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
	serverCtx.ExecRequest.Continue()

	// Copy the SSH channel to and from the anonymous pipes
	s.errCh = make(chan error, 2)
	go func() {
		defer chReadPipeIn.Close()

		_, err := io.Copy(chReadPipeIn, s.ch)
		s.errCh <- err
	}()
	go func() {
		defer chWritePipeOut.Close()

		_, err := io.Copy(s.ch, chWritePipeOut)
		s.errCh <- err
	}()

	go func() {
		defer auditPipeOut.Close()

		var buf bytes.Buffer
		_, err := io.Copy(&buf, auditPipeOut)
		s.errCh <- err

		var sftpEvents []*apievents.SFTP
		err = json.Unmarshal(buf.Bytes(), &sftpEvents)
		if err != nil {
			s.log.WithError(err).Error("Failed to unmarshal SFTP events.")
			return
		}

		// Create common fields for events
		serverMeta := apievents.ServerMetadata{
			ServerID:        serverCtx.GetServer().HostUUID(),
			ServerHostname:  serverCtx.GetServer().GetInfo().GetHostname(),
			ServerNamespace: serverCtx.GetServer().GetNamespace(),
		}
		sessionMeta := apievents.SessionMetadata{
			SessionID: string(serverCtx.SessionID()),
			// TODO: no idea what this should be set to
			WithMFA: serverCtx.Identity.Certificate.Extensions[teleport.CertExtensionMFAVerified],
		}
		userMeta := serverCtx.Identity.GetUserMetadata()
		connectionMeta := apievents.ConnectionMetadata{
			RemoteAddr: serverConn.RemoteAddr().String(),
			LocalAddr:  serverConn.LocalAddr().String(),
		}

		for _, event := range sftpEvents {
			event.Metadata.ClusterName = serverCtx.ClusterName
			event.ServerMetadata = serverMeta
			event.SessionMetadata = sessionMeta
			event.UserMetadata = userMeta
			event.ConnectionMetadata = connectionMeta

			if err := serverCtx.GetServer().EmitAuditEvent(ctx, event); err != nil {
				log.WithError(err).Warn("Failed to emit SFTP event.")
			}
		}
	}()

	return nil
}

func (s *sftpSubsys) Wait() error {
	waitErr := s.sftpCmd.Wait()
	s.log.Debug("SFTP process finished")

	errs := []error{waitErr}
	for i := 0; i < 3; i++ {
		err := <-s.errCh
		if err != nil && !utils.IsOKNetworkError(err) {
			s.log.WithError(err).Warn("Connection problem.")
			errs = append(errs, err)
		}
	}
	errs = append(errs, s.ch.Close())

	return trace.NewAggregate(errs...)
}
