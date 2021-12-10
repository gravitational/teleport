/*
Copyright 2016 Gravitational, Inc.

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

package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client/escape"
	"github.com/gravitational/teleport/lib/client/terminal"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

const (
	ctrlCharC byte = 0x03
	ctrlCharZ byte = 0x26
)

type NodeSession struct {
	// namespace is a session this namespace belongs to
	namespace string

	// id is the Teleport session ID
	id session.ID

	// env is the environment variables that need to be created
	// on the server for this session
	env map[string]string

	// nodeClient is the parent of this session: the client connected to an
	// SSH node
	nodeClient *NodeClient

	// closer is used to simultaneously close all goroutines created by
	// this session.
	closer *utils.CloseBroadcaster

	// closeWait is used to wait for cleanup-related goroutines created by
	// this session to close.
	closeWait *sync.WaitGroup

	ExitMsg string

	enableEscapeSequences bool

	terminal *terminal.Terminal
}

// newSession creates a new Teleport session with the given remote node
// if 'joinSessin' is given, the session will join the existing session
// of another user
func newSession(client *NodeClient,
	joinSession *session.Session,
	env map[string]string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	legacyID bool,
	enableEscapeSequences bool,
) (*NodeSession, error) {
	// Initialize the terminal. Note that at this point, we don't know if this
	// will be an interactive session, so we don't yet enable either raw mode
	// or raw input.
	term, err := terminal.New(stdin, stdout, stderr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if env == nil {
		env = make(map[string]string)
	}

	ns := &NodeSession{
		env:                   env,
		nodeClient:            client,
		namespace:             client.Namespace,
		closer:                utils.NewCloseBroadcaster(),
		closeWait:             &sync.WaitGroup{},
		enableEscapeSequences: enableEscapeSequences,
		terminal:              term,
	}
	// if we're joining an existing session, we need to assume that session's
	// existing/current terminal size:
	if joinSession != nil {
		ns.id = joinSession.ID
		ns.namespace = joinSession.Namespace
		tsize := joinSession.TerminalParams.Winsize()

		if ns.terminal.IsAttached() {
			err = ns.terminal.Resize(int16(tsize.Width), int16(tsize.Height))
			if err != nil {
				log.Error(err)
			}

		}
		// new session!
	} else {
		sid, ok := ns.env[sshutils.SessionEnvVar]
		if !ok {
			// DELETE IN: 4.1.0.
			//
			// Always send UUIDv4 after 4.1.
			if legacyID {
				sid = string(session.NewLegacyID())
			} else {
				sid = string(session.NewID())
			}

		}
		ns.id = session.ID(sid)
	}

	ns.env[sshutils.SessionEnvVar] = string(ns.id)

	// Close the Terminal when finished.
	ns.closeWait.Add(1)
	go func() {
		defer ns.closeWait.Done()

		<-ns.closer.C
		if isFIPS() {
			if err := ns.terminal.Clear(); err != nil {
				log.Warnf("Failed to clear screen: %v.", err)
			}
		}
		ns.terminal.Close()
	}()

	return ns, nil
}

func (ns *NodeSession) NodeClient() *NodeClient {
	return ns.nodeClient
}

func (ns *NodeSession) regularSession(ctx context.Context, callback func(s *ssh.Session) error) error {
	session, err := ns.createServerSession(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	session.Stdout = ns.terminal.Stdout()
	session.Stderr = ns.terminal.Stderr()
	session.Stdin = ns.terminal.Stdin()
	return trace.Wrap(callback(session))
}

type interactiveCallback func(serverSession *ssh.Session, shell io.ReadWriteCloser) error

func (ns *NodeSession) createServerSession(ctx context.Context) (*ssh.Session, error) {
	sess, err := ns.nodeClient.Client.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// request x11 forwarding for the session if the client requested it.
	if ns.nodeClient.TC.Config.X11ForwardingEnabled {
		if err := x11.RequestX11Forwarding(ctx, sess, ns.nodeClient.Client, false,
			ns.nodeClient.TC.Config.X11ForwardingTrusted,
			ns.nodeClient.TC.Config.X11ForwardingTimeout,
		); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// pass language info into the remote session.
	evarsToPass := []string{"LANG", "LANGUAGE"}
	for _, evar := range evarsToPass {
		if value := os.Getenv(evar); value != "" {
			err = sess.Setenv(evar, value)
			if err != nil {
				log.Warn(err)
			}
		}
	}
	// pass environment variables set by client
	for key, val := range ns.env {
		err = sess.Setenv(key, val)
		if err != nil {
			log.Warn(err)
		}
	}

	// if agent forwarding was requested (and we have a agent to forward),
	// forward the agent to endpoint.
	tc := ns.nodeClient.Proxy.teleportClient
	targetAgent := selectKeyAgent(tc)

	if targetAgent != nil {
		log.Debugf("Forwarding Selected Key Agent")
		err = agent.ForwardToAgent(ns.nodeClient.Client, targetAgent)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = agent.RequestAgentForwarding(sess)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return sess, nil
}

// selectKeyAgent picks the appropriate key agent for forwarding to the
// server, if any.
func selectKeyAgent(tc *TeleportClient) agent.Agent {
	switch tc.ForwardAgent {
	case ForwardAgentYes:
		log.Debugf("Selecting system key agent.")
		return tc.localAgent.sshAgent
	case ForwardAgentLocal:
		log.Debugf("Selecting local Teleport key agent.")
		return tc.localAgent.Agent
	default:
		log.Debugf("No Key Agent selected.")
		return nil
	}
}

// interactiveSession creates an interactive session on the remote node, executes
// the given callback on it, and waits for the session to end
func (ns *NodeSession) interactiveSession(ctx context.Context, callback interactiveCallback) error {
	// determine what kind of a terminal we need
	termType := os.Getenv("TERM")
	if termType == "" {
		termType = teleport.SafeTerminalType
	}
	// create the server-side session:
	sess, err := ns.createServerSession(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	// allocate terminal on the server:
	remoteTerm, err := ns.allocateTerminal(termType, sess)
	if err != nil {
		return trace.Wrap(err)
	}
	defer remoteTerm.Close()

	// call the passed callback and give them the established
	// ssh session:
	if err := callback(sess, remoteTerm); err != nil {
		return trace.Wrap(err)
	}

	if ns.terminal.IsAttached() {
		// Put the terminal into raw mode. Note that this must be done before
		// pipeInOut() as it may replace streams.
		ns.terminal.InitRaw(true)

		// Catch term signals, but only if we're attached to a real terminal
		ns.watchSignals(remoteTerm)
	}

	// start piping input into the remote shell and pipe the output from
	// the remote shell into stdout:
	ns.pipeInOut(remoteTerm)

	// wait for the session to end
	<-ns.closer.C

	// Wait for any cleanup tasks (particularly terminal reset on Windows).
	ns.closeWait.Wait()
	return sess.Wait()
}

// allocateTerminal creates (allocates) a server-side terminal for this session.
func (ns *NodeSession) allocateTerminal(termType string, s *ssh.Session) (io.ReadWriteCloser, error) {
	var err error

	// read the size of the terminal window:
	width := teleport.DefaultTerminalWidth
	height := teleport.DefaultTerminalHeight
	if ns.terminal.IsAttached() {
		realWidth, realHeight, err := ns.terminal.Size()
		if err != nil {
			log.Error(err)
		} else {
			width = int(realWidth)
			height = int(realHeight)
		}
	}

	// ... and request a server-side terminal of the same size:
	err = s.RequestPty(termType,
		height,
		width,
		ssh.TerminalModes{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	writer, err := s.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reader, err := s.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stderr, err := s.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ns.terminal.IsAttached() {
		go ns.updateTerminalSize(s)
	}
	go func() {
		if _, err := io.Copy(os.Stderr, stderr); err != nil {
			log.Debugf("Error reading remote STDERR: %v", err)
		}
	}()
	return utils.NewPipeNetConn(
		reader,
		writer,
		utils.MultiCloser(writer, s, ns.closer),
		&net.IPAddr{},
		&net.IPAddr{},
	), nil
}

func (ns *NodeSession) updateTerminalSize(s *ssh.Session) {
	terminalEvents := ns.terminal.Subscribe()

	lastWidth, lastHeight, err := ns.terminal.Size()
	if err != nil {
		log.Errorf("Unable to get window size: %v", err)
		return
	}

	// Sync the local terminal with size received from the remote server every
	// two seconds. If we try and do it live, synchronization jitters occur.
	tickerCh := time.NewTicker(defaults.TerminalResizePeriod)
	defer tickerCh.Stop()

	for {
		select {
		// The client updated the size of the local PTY. This change needs to occur
		// on the server side PTY as well.
		case event, ok := <-terminalEvents:
			if !ok {
				return
			}

			// Make sure this is a resize event.
			if _, ok := event.(terminal.ResizeEvent); !ok {
				continue
			}

			currWidth, currHeight, err := ns.terminal.Size()
			if err != nil {
				log.Warnf("Unable to get window size: %v.", err)
				continue
			}

			// Terminal size has not changed, don't do anything.
			if currHeight == lastHeight && currWidth == lastWidth {
				continue
			}

			// Send the "window-change" request over the channel.
			_, err = s.SendRequest(
				sshutils.WindowChangeRequest,
				false,
				ssh.Marshal(sshutils.WinChangeReqParams{
					W: uint32(currWidth),
					H: uint32(currHeight),
				}))
			if err != nil {
				log.Warnf("Unable to send %v reqest: %v.", sshutils.WindowChangeRequest, err)
				continue
			}

			log.Debugf("Updated window size from (%d, %d) to (%d, %d) due to SIGWINCH.", lastWidth, lastHeight, currWidth, currHeight)

			lastWidth, lastHeight = currWidth, currHeight

		// Extract "resize" events in the stream and store the last window size.
		case event := <-ns.nodeClient.TC.EventsChannel():
			// Only "resize" events are important to tsh, all others can be ignored.
			if event.GetType() != events.ResizeEvent {
				continue
			}

			terminalParams, err := session.UnmarshalTerminalParams(event.GetString(events.TerminalSize))
			if err != nil {
				log.Warnf("Unable to unmarshal terminal parameters: %v.", err)
				continue
			}

			lastSize := terminalParams.Winsize()
			lastWidth = int16(lastSize.Width)
			lastHeight = int16(lastSize.Height)
			log.Debugf("Recevied window size %v from node in session %v.", lastSize, event.GetString(events.SessionEventID))

		// Update size of local terminal with the last size received from remote server.
		case <-tickerCh.C:
			// Get the current size of the terminal and the last size report that was
			// received.
			currWidth, currHeight, err := ns.terminal.Size()
			if err != nil {
				log.Warnf("Unable to get current terminal size: %v.", err)
				continue
			}

			// Terminal size has not changed, don't do anything.
			if currWidth == lastWidth && currHeight == lastHeight {
				continue
			}

			// This changes the size of the local PTY. This will re-draw what's within
			// the window.
			err = ns.terminal.Resize(lastWidth, lastHeight)
			if err != nil {
				log.Warnf("Unable to update terminal size: %v.", err)
				continue
			}

			log.Debugf("Updated window size from (%d, %d) to (%d, %d) due to remote window change.", currWidth, currHeight, lastWidth, lastHeight)
		case <-ns.closer.C:
			return
		}
	}
}

// runShell executes user's shell on the remote node under an interactive session
func (ns *NodeSession) runShell(ctx context.Context, callback ShellCreatedCallback) error {
	return ns.interactiveSession(ctx, func(s *ssh.Session, shell io.ReadWriteCloser) error {
		// start the shell on the server:
		if err := s.Shell(); err != nil {
			return trace.Wrap(err)
		}
		// call the client-supplied callback
		if callback != nil {
			exit, err := callback(s, ns.NodeClient().Client, shell)
			if exit {
				return trace.Wrap(err)
			}
		}
		return nil
	})
}

// runCommand executes a "exec" request either in interactive mode (with a
// TTY attached) or non-intractive mode (no TTY).
func (ns *NodeSession) runCommand(ctx context.Context, cmd []string, callback ShellCreatedCallback, interactive bool) error {
	// If stdin is not a terminal, refuse to allocate terminal on the server and
	// fallback to non-interactive mode
	if interactive && !ns.terminal.IsAttached() {
		interactive = false
		fmt.Fprintf(os.Stderr, "TTY will not be allocated on the server because stdin is not a terminal\n")
	}

	// Start a interactive session ("exec" request with a TTY).
	//
	// Note that because a TTY was allocated, the terminal is in raw mode and any
	// keyboard based signals will be propogated to the TTY on the server which is
	// where all signal handling will occur.
	if interactive {
		return ns.interactiveSession(ctx, func(s *ssh.Session, term io.ReadWriteCloser) error {
			err := s.Start(strings.Join(cmd, " "))
			if err != nil {
				return trace.Wrap(err)
			}
			if callback != nil {
				exit, err := callback(s, ns.NodeClient().Client, term)
				if exit {
					return trace.Wrap(err)
				}
			}
			return nil
		})
	}

	// Start a non-interactive session ("exec" request without TTY).
	//
	// Note that for non-interactive sessions upon receipt of SIGINT the client
	// should send a SSH_MSG_DISCONNECT and shut itself down as gracefully as
	// possible. This is what the RFC recommends and what OpenSSH does:
	//
	//  * https://tools.ietf.org/html/rfc4253#section-11.1
	//  * https://github.com/openssh/openssh-portable/blob/05046d907c211cb9b4cd21b8eff9e7a46cd6c5ab/clientloop.c#L1195-L1444
	//
	// Unfortunately at the moment the Go SSH library Teleport uses does not
	// support sending SSH_MSG_DISCONNECT. Instead we close the SSH channel and
	// SSH client, and try and exit as gracefully as possible.
	return ns.regularSession(ctx, func(s *ssh.Session) error {
		var err error

		runContext, cancel := context.WithCancel(ctx)
		go func() {
			defer cancel()
			err = s.Run(strings.Join(cmd, " "))
		}()

		select {
		// Run returned a result, return that back to the caller.
		case <-runContext.Done():
			return trace.Wrap(err)
		// The passed in context timed out. This is often due to the user hitting
		// Ctrl-C.
		case <-ctx.Done():
			err = s.Close()
			if err != nil {
				log.Debugf("Unable to close SSH channel: %v", err)
			}
			err = ns.NodeClient().Client.Close()
			if err != nil {
				log.Debugf("Unable to close SSH client: %v", err)
			}
			return trace.ConnectionProblem(ctx.Err(), "connection canceled")
		}
	})
}

// watchSignals register UNIX signal handlers and properly terminates a remote shell session
// must be called as a goroutine right after a remote shell is created
func (ns *NodeSession) watchSignals(shell io.Writer) {
	// First, watch for standard cross platform signals (SIGTERM, SIGINT).

	// Catch SIGTERM and close the session.
	exitSignals := make(chan os.Signal, 1)
	signal.Notify(exitSignals, syscall.SIGTERM)
	go func() {
		defer ns.closer.Close()

		select {
		case <-exitSignals:
			return
		case <-ns.closer.C:
			return
		}
	}()

	// Catch Ctrl-C/SIGINT.
	ctrlCSignal := make(chan os.Signal, 1)
	signal.Notify(ctrlCSignal, syscall.SIGINT)
	go func() {
		for {
			select {
			case <-ctrlCSignal:
				_, err := shell.Write([]byte{ctrlCharC})
				if err != nil {
					log.Errorf(err.Error())
				}
			case <-ns.closer.C:
				return
			}
		}
	}()

	// Then, use Terminal events for SIGTSTP, which is not supported on
	// Windows. (Windows emits the Ctrl+Z sequence directly.)
	events := ns.terminal.Subscribe()
	go func() {
		for event := range events {
			if _, ok := event.(terminal.StopEvent); ok {
				_, err := shell.Write([]byte{ctrlCharZ})
				if err != nil {
					log.Errorf(err.Error())
				}
			}
		}
	}()
}

// pipeInOut launches two goroutines: one to pipe the local input into the remote shell,
// and another to pipe the output of the remote shell into the local output
func (ns *NodeSession) pipeInOut(shell io.ReadWriteCloser) {
	// copy from the remote shell to the local output
	go func() {
		defer ns.closer.Close()
		_, err := io.Copy(ns.terminal.Stdout(), shell)
		if err != nil {
			log.Errorf(err.Error())
		}
	}()
	// copy from the local input to the remote shell:
	go func() {
		defer ns.closer.Close()
		buf := make([]byte, 128)

		stdin := ns.terminal.Stdin()
		if ns.terminal.IsAttached() && ns.enableEscapeSequences {
			stdin = escape.NewReader(stdin, ns.terminal.Stderr(), func(err error) {
				switch err {
				case escape.ErrDisconnect:
					fmt.Fprintf(ns.terminal.Stderr(), "\r\n%v\r\n", err)
				case escape.ErrTooMuchBufferedData:
					fmt.Fprintf(ns.terminal.Stderr(), "\r\nerror: %v\r\nremote peer may be unreachable, check your connectivity\r\n", trace.Wrap(err))
				default:
					fmt.Fprintf(ns.terminal.Stderr(), "\r\nerror: %v\r\n", trace.Wrap(err))
				}
				ns.closer.Close()
			})
		}
		for {
			n, err := stdin.Read(buf)
			if err != nil {
				fmt.Fprintf(ns.terminal.Stderr(), "\r\n%v\r\n", trace.Wrap(err))
				return
			}

			if n > 0 {
				_, err = shell.Write(buf[:n])
				if err != nil {
					ns.ExitMsg = err.Error()
					return
				}
			}
		}
	}()
}

func (ns *NodeSession) Close() error {
	if ns.closer != nil {
		ns.closer.Close()
	}
	if ns.closeWait != nil {
		ns.closeWait.Wait()
	}
	return nil
}
