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
		recvFrom:  make(chan tdp.Message),
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

	// recvFrom is the channel from which we will receive TDP messages from the RDP client
	// (which will have translated them from RDP).
	recvFrom chan tdp.Message

	// sendTo is the channel to which we will send TDP messages sent to us from the browser
	// (which the RDP client will translate to RDP and send to the Windows host).
	sendTo chan tdp.Message

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
	// Tell the RDP client to start translating messages sent to it by the
	// Windows host from RDP into TDP, and then sending them to the recvFrom channel.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.rdpc.Close()
		defer s.log.Info("RDP --> TDP streaming finished")
		defer close(s.recvFrom) // TODO(isaiah): perhaps bad practice to close here. Once we figure out what we're doing with StartSendingTo, we may want to move channel closure to inside that call-chain.

		// Call StartSendingTo with the recvFrom channel, will tells the RDP client
		// to begin translating incoming RDP messages to TDP and sending them to the channel.
		// TODO(isaiah): this goroutine may never return because StartSendingTo waits on read_rdp_output (https://github.com/gravitational/teleport/blob/e3fa4d7f00445c1e4149ff82090820b38117ad75/lib/srv/desktop/rdp/rdpclient/src/lib.rs#L343),
		// which itself may never break from the while loop in read_rdp_output_inner, because read_rdp_output_inner calls handle_bitmap which calls lib/srv/desktop/rdp/rdpclient/client.go::handleBitmap. See the "TODO(isaiah)" in that function for the
		// reasoning behind that suspicion.
		if err := s.rdpc.StartSendingTo(s.recvFrom); err != nil {
			s.log.Error(err)
			return
		}
	}()

	// Start recieving on the recvFrom channel and forwarding to the TDP connection.
	// TODO(isaiah): Do we need more s.wg logic here?
	go func() {
		// Receive loop is broken by channel close in the goroutine above this one (TODO(isaiah): this comment may need to be revised)
		for msg := range s.recvFrom {
			if err := s.tdpConn.OutputMessage(msg); err != nil {
				s.log.WithError(err).Warning("Failed to forward TDP output message: %+v", msg)
			}
		}
	}()

	// Start fielding TDP messages coming from the browser and sending them to the sendTo channel.
	// TODO(isaiah): Do we need more s.wg logic here?
	go func() {
		for {
			// TODO(isaiah): this goroutine only returns when tdpConn.InputMessage() gives us an error,
			// which is only one of the mechanisms by which it's corresponding goroutine returned in Andrew's
			// version (https://github.com/gravitational/teleport/blob/675fb3dc09f1e95b58dedb19021599ec84bb1b22/lib/srv/desktop/rdp/rdpclient/client.go#L251-L258).
			// Is this sufficient to prevent a leak?
			msg, err := s.tdpConn.InputMessage()
			if err != nil {
				s.log.WithError(err).Warning("Failed reading TDP input message")
				return
			}

			s.sendTo <- msg
		}
	}()

	// Tell the RDP client to start receiving TDP messages on the sendTo channel, translating them
	// to RDP, and forwarding them to the Windows host.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.rdpc.Close()
		defer s.log.Info("TDP --> RDP input streaming finished")

		if err := s.rdpc.StartReceivingFrom(s.sendTo); err != nil {
			s.log.Error(err)
			return
		}
	}()
}

// wait blocks until streaming is finished and then runs cleanup.
func (s *session) wait() {
	s.wg.Wait()
	s.rdpc.Cleanup()
}
