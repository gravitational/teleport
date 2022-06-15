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
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/gravitational/teleport"
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
			trace.Component:       teleport.ComponentSubsystemSFTP,
			trace.ComponentFields: map[string]string{},
		}),
	}, nil
}

func (s *sftpSubsys) Start(ctx context.Context, _ *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, serverCtx *srv.ServerContext) error {
	s.ch = ch

	err := req.Reply(true, nil)
	if err != nil {
		return trace.Wrap(err)
	}

	// Create two sets of anonymous pipes that we can use to give the
	// child process access to the SSH channel
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

	executable, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	serverCtx.ExecRequest, err = srv.NewExecRequest(serverCtx, executable+" sftp")
	if err != nil {
		return trace.Wrap(err)
	}
	s.sftpCmd, err = srv.ConfigureCommand(serverCtx, chReadPipeOut, chWritePipeIn)
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

	return nil
}

func (s *sftpSubsys) Wait() error {
	waitErr := s.sftpCmd.Wait()
	s.log.Debug("SFTP process finished")

	errs := []error{waitErr}
	for i := 0; i < 2; i++ {
		err := <-s.errCh
		if err != nil && !utils.IsOKNetworkError(err) {
			s.log.WithError(err).Warn("Connection problem.")
			errs = append(errs, err)
		}
	}
	errs = append(errs, s.ch.Close())

	return trace.NewAggregate(errs...)
}
