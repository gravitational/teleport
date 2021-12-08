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
		sessionID:      sessionID,
		tdpConn:        tdpConn,
		rdpc:           rdpc,
		recvFromRdpCli: make(chan tdp.Message),
		log:            log,
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

	// recvFromRdpCli is the channel on which we will receive TDP messages from the RDP client to forward to the browser.
	// (The RDP client will have translated them from RDP).
	recvFromRdpCli chan tdp.Message

	// sendToRdpCli is the channel to which we will send TDP messages sent to us by the browser to. The RDP client
	// will then translate them to RDP and send them to the Windows host.
	sendToRdpCli chan tdp.Message

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
		msg, err := s.tdpConn.Read()
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
		msg, err := s.tdpConn.Read()
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

// Tell the RDP client to start translating messages sent to it by the
// Windows host from RDP into TDP, and then sending them to our recvFromRdpCli channel.
func (s *session) streamFromRdpCliToHere() {
	s.wg.Add(1)
	s.log.Debug("Windows host --> RDP client --> TDP session streaming started")

	defer s.wg.Done()
	defer s.rdpc.Close()
	defer s.tdpConn.Close()
	defer s.log.Debug("Windows host --> RDP client --> TDP session streaming finished")

	// Call StartStreamingRDPtoTDP with the recvFromRdpCli channel, will tells the RDP client to begin
	// translating incoming RDP messages (from the Windows host) to TDP and sending them to the channel.
	// This will return on an RDP read error or when s.rdpc.Close() is called.
	if err := s.rdpc.StartStreamingRDPtoTDP(s.recvFromRdpCli); err != nil {
		s.log.Error(err)
	}
}

// Takes the stream of TDP messages (initiated by streamFromRdpCliToHere) and forwards them to the browser.
func (s *session) streamFromHereToBrowser() {
	s.wg.Add(1)
	s.log.Debug("TDP session --> browser streaming started")

	defer s.wg.Done()
	defer s.rdpc.Close()
	defer s.tdpConn.Close()
	defer s.log.Debug("TDP session --> browser streaming finished")

	// This loop is broken by an error or the channel being closed by StartStreamingRDPtoTDP.
	for msg := range s.recvFromRdpCli {
		if err := s.tdpConn.Write(msg); err != nil {
			s.log.WithError(err).Warning("Failed to forward TDP output message: %+v", msg)
			break
		}
	}
}

// Start fielding TDP messages coming from the browser and sending them to the sendToRdpCli channel.
func (s *session) streamFromBrowserToRdpCli() {
	s.wg.Add(1)
	s.log.Debug("browser --> TDP session --> RDP client streaming started")

	defer s.wg.Done()
	defer s.rdpc.Close()
	defer s.tdpConn.Close()
	defer s.log.Debug("browser --> TDP session --> RDP client streaming finished")

	// TODO: currently the only way we exit from this loop is by the underlying tls connection being closed,
	// which causes Read() to return an error. We can fix this by adding a disconnect message to TDP.
	for {
		msg, err := s.tdpConn.Read()
		if err != nil {
			s.log.WithError(err).Warning("Failed reading TDP input message")
			break
		}

		s.sendToRdpCli <- msg
	}

	close(s.sendToRdpCli)
}

// Tell the RDP client to start receiving TDP messages on the sendToRdpCli channel, translating them
// to RDP, and forwarding them to the Windows host.
func (s *session) streamFromRdpCliToWindowsHost() {
	s.wg.Add(1)
	s.log.Debug("RDP client --> Windows host streaming started")

	defer s.wg.Done()
	defer s.rdpc.Close()
	defer s.tdpConn.Close()
	defer s.log.Debug("RDP client --> Windows host streaming finished")

	// Call StartStreamingTDPtoRDP with the sendToRdpCli channel, which tells the RDP client to begin translating
	// incoming TDP messages (from the browser) to RDP and sending them to the Windows host.
	// We are responsible for closing the channel we pass to StartStreamingTDPtoRDP, which is done in streamFromBrowserToRdpCli.
	if err := s.rdpc.StartStreamingTDPtoRDP(s.sendToRdpCli); err != nil {
		s.log.Error(err)
	}
}

// start kicks off goroutines for streaming input and output between the TDP connection and
// RDP client and returns right away. Use Wait to wait for them to finish.
func (s *session) start() {
	go s.streamFromRdpCliToHere()
	go s.streamFromHereToBrowser()

	go s.streamFromBrowserToRdpCli()
	go s.streamFromRdpCliToWindowsHost()
}

// wait blocks until streaming is finished and then runs cleanup.
func (s *session) wait() {
	s.wg.Wait()
	s.rdpc.Cleanup()
}
