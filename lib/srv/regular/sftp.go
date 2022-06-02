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
	"errors"
	"io"

	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/trace"
)

type sftpSubsys struct {
	sftpSrv *sftp.Server
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

func (s *sftpSubsys) Start(ctx context.Context, _ *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, _ *srv.ServerContext) error {
	err := req.Reply(true, nil)
	if err != nil {
		return trace.Wrap(err, "error replying to subsystem request")
	}

	sftpSrv, err := sftp.NewServer(ch, sftp.WithDebug(s.log.WriterLevel(logrus.DebugLevel)))
	if err != nil {
		return trace.Wrap(err, "error creating SFTP server")
	}
	s.sftpSrv = sftpSrv

	s.log.Debug("starting SFTP server")
	err = s.sftpSrv.Serve()
	s.log.Debug("SFTP server stopped")
	if errors.Is(err, io.EOF) {
		err = nil
	}

	return err
}

func (s *sftpSubsys) Wait() error {
	s.log.Debug("closing SFTP server")
	return s.sftpSrv.Close()
}
