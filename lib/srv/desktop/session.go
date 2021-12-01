/*
Copyright 2021 Gravitational, Inc.

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

package desktop

import (
	"context"

	sshsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/rdpclient"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

func newSession(ctx context.Context, sessionID sshsession.ID, tdpConn *tdp.Conn, rdpc *rdpclient.Client, log logrus.FieldLogger) (*session, error) {
	s := &session{
		sessionID: sessionID,
		tdpConn:   tdpConn,
		rdpc:      rdpc,
		log:       log,
	}

	u, err := s.waitForUsername()
	if err != nil {
		return nil, err
	}

	sc, err := s.waitForClientSize()
	if err != nil {
		return nil, err
	}

	if err := s.rdpc.Connect(ctx, u, sc); err != nil {
		return nil, err
	}

	return s, nil
}

// session represents desktop session
type session struct {
	sessionID sshsession.ID

	// tdpConn is the TDP server connection for sending and receiving
	// TDP messages to and from the TDP client (browser).
	tdpConn *tdp.Conn

	// rdpc is the RDP client for sending and receiving RDP messages
	// to and from the RDP server (windows host).
	rdpc *rdpclient.Client

	log logrus.FieldLogger
}

// waitForUsername waits for the tdp connection to recieve a username.
// Returns a non-nil error if another message is recieved.
func (s *session) waitForUsername() (*tdp.ClientUsername, error) {
	for {
		msg, err := s.tdpConn.InputMessage()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		u, ok := msg.(tdp.ClientUsername)
		if !ok {
			return nil, trace.BadParameter("Expected ClientUsername message, got %T", msg)
		}
		s.log.Debugf("Got username %q", u.Username)
		return &u, nil
	}
}

// waitForClientSize waits for the tdp connection to receive a client size.
// Returns a non-nil error if another message is recieved.
func (s *session) waitForClientSize() (*tdp.ClientScreenSpec, error) {
	for {
		msg, err := s.tdpConn.InputMessage()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sc, ok := msg.(tdp.ClientScreenSpec)
		if !ok {
			return nil, trace.BadParameter("Expected ClientScreenSpec message, got %T", msg)
		}
		s.log.Debugf("Got RDP screen size %dx%d", sc.Width, sc.Height)
		return &sc, nil
	}
}
