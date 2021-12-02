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
	"sync"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	sshsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/rdpclient"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

// newSession creates a new session object, waits for initial TDP messages,
// checks that the user has permission to create a new session with the requested desktop,
// and then if everything checks out, creates the RDP connection.
func newSession(ctx context.Context, authCtx *auth.Context, desktop types.WindowsDesktop, sessionID sshsession.ID, tdpConn *tdp.Conn, rdpc *rdpclient.Client, log logrus.FieldLogger) (*session, error) {
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

	// Save the username for audit logging and other uses elsewhere.
	s.username = u.Username

	// Check that user has permission to create a new session with the requested desktop.
	if err := authCtx.Checker.CheckAccess(
		desktop,
		services.AccessMFAParams{Verified: true},
		services.NewWindowsLoginMatcher(u.Username)); err != nil {
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

// session represents a desktop session.
type session struct {
	sessionID sshsession.ID

	// tdpConn is the TDP server connection for sending and receiving
	// TDP messages to and from the TDP client (browser).
	tdpConn *tdp.Conn

	// rdpc is the RDP client for sending and receiving RDP messages
	// to and from the RDP server (windows host).
	rdpc *rdpclient.Client

	// wg is used to wait for the input/output streaming
	// goroutines to complete.
	wg sync.WaitGroup

	// username saves the initial username from the initial TDP ClientUsername message for later use.
	username string

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

// start kicks off goroutines for streaming input and output between the TDP connection and
// RDP client and returns right away. Use Wait to wait for them to finish.
func (s *session) start() {
	// Stream RDP output.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.rdpc.Close()
		defer s.log.Info("RDP --> TDP streaming finished")

		ch := make(chan tdp.Message)

		// Spawn goroutine for recieving from the ch
		// and forwarding to the TDP connection.
		go func() {
			for msg := range ch {
				if err := s.tdpConn.OutputMessage(msg); err != nil {
					s.log.WithError(err).Warning("Failed to forward TDP output message: %+v", msg)
				}
			}
		}()

		// Call StartReceiving with the ch, will tells the RDP client
		// to begin translating incoming RDP messages to TDP and sending
		// them to the ch.
		if err := s.rdpc.StartReceiving(ch); err != nil {
			s.log.Error(err)
			return
		}
	}()

	// Stream user input messages.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.rdpc.Close()
		defer s.log.Info("TDP --> RDP input streaming finished")

		for {
			msg, err := s.tdpConn.InputMessage()
			if err != nil {
				s.log.WithError(err).Warning("Failed reading TDP input message")
				continue
			}

			if err := s.rdpc.Send(msg); err != nil {
				s.log.Warning(err)
			}
		}
	}()
}

// wait blocks until streaming is finished and then runs cleanup.
func (s *session) wait() {
	s.wg.Wait()
	s.rdpc.Cleanup()
}
