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

func newSftpSubsys() (*sftpSubsys, error) {
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

	chrr, chrw, err := os.Pipe()
	if err != nil {
		return trace.Wrap(err)
	}
	chwr, chww, err := os.Pipe()
	if err != nil {
		return trace.Wrap(err)
	}

	executable, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	serverCtx.ExecRequest, err = srv.NewExecRequest(serverCtx, executable+" sftp")
	if err != nil {
		return trace.Wrap(err)
	}
	s.sftpCmd, err = srv.ConfigureCommand(serverCtx, chrr, chww)
	if err != nil {
		return trace.Wrap(err)
	}
	s.sftpCmd.Stdout = os.Stdout
	s.sftpCmd.Stderr = os.Stderr

	err = s.sftpCmd.Start()
	if err != nil {
		return trace.Wrap(err)
	}
	// TODO: put in cgroup?
	serverCtx.ExecRequest.Continue()

	errCh := make(chan error, 2)
	go func() {
		defer chrw.Close()

		_, err := io.Copy(chrw, s.ch)
		errCh <- err
	}()
	go func() {
		defer chwr.Close()

		_, err := io.Copy(s.ch, chwr)
		errCh <- err
	}()

	return nil
}

func (s *sftpSubsys) Wait() error {
	waitErr := s.sftpCmd.Wait()
	closeErr := s.ch.Close()

	inCopyErr := <-s.errCh
	outCopyErr := <-s.errCh

	copyErr := trace.NewAggregate(inCopyErr, outCopyErr)

	return trace.NewAggregate(copyErr, waitErr, closeErr)
}
