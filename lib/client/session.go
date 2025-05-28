/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/escape"
	"github.com/gravitational/teleport/lib/client/terminal"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	ctrlCharC byte = 0x03
	ctrlCharZ byte = 0x26
)

type NodeSession struct {
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

	// forceDisconnect if we should immediately disconnect upon finish instead of waiting for the remote status.
	forceDisconnect atomic.Bool

	// shouldClearOnExit marks whether or not the terminal should be cleared
	// when the session ends.
	shouldClearOnExit bool
	// clientXAuthEntry contains xauth data which provides
	// access to the client's local XServer.
	clientXAuthEntry *x11.XAuthEntry
	// spoofedXAuthEntry is a copy of the client's xauth data with a
	// spoofed cookie. This cookie will be used to authenticate server
	// requests without exposing the client's cookie.
	spoofedXAuthEntry *x11.XAuthEntry
	// x11RefuseTime is an optional time at which X11 channel
	// requests using the xauth cookie will be rejected.
	x11RefuseTime time.Time
}

// newSession creates a new Teleport session with the given remote node
// if 'joinSession' is given, the session will join the existing session
// of another user
func newSession(ctx context.Context,
	client *NodeClient,
	joinSession types.SessionTracker,
	env map[string]string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
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
		closer:                utils.NewCloseBroadcaster(),
		closeWait:             &sync.WaitGroup{},
		enableEscapeSequences: enableEscapeSequences,
		terminal:              term,
		shouldClearOnExit:     client.FIPSEnabled || isFIPS(),
	}
	// if we're joining an existing session, we need to assume that session's
	// existing/current terminal size:
	if joinSession != nil {
		sessionID := joinSession.GetSessionID()
		terminalSize, err := client.GetRemoteTerminalSize(ctx, sessionID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ns.id = session.ID(sessionID)

		if ns.terminal.IsAttached() {
			err = ns.terminal.Resize(int16(terminalSize.Width), int16(terminalSize.Height))
			if err != nil {
				log.ErrorContext(ctx, "Failed to resize terminal", "error", err)
			}

		}
		// new session!
	} else {
		// TODO(capnspacehook): DELETE IN 17.0.0
		// clients shouldn't set TELEPORT_SESSION when they aren't joining
		// a session, and won't need to once all supported Proxy/Node
		// versions set the session ID for new sessions
		sid, ok := ns.env[sshutils.SessionEnvVar]
		if !ok {
			sid = string(session.NewID())
		}
		ns.id = session.ID(sid)
	}

	ns.env[sshutils.SessionEnvVar] = string(ns.id)

	// Close the Terminal when finished.
	ns.closeWait.Add(1)
	go func() {
		defer ns.closeWait.Done()

		<-ns.closer.C

		if ns.shouldClearOnExit {
			if err := ns.terminal.Clear(); err != nil {
				log.WarnContext(ctx, "Failed to clear screen", "error", err)
			}
		}
		ns.terminal.Close()
	}()

	return ns, nil
}

func (ns *NodeSession) NodeClient() *NodeClient {
	return ns.nodeClient
}

func (ns *NodeSession) regularSession(ctx context.Context, chanReqCallback tracessh.ChannelRequestCallback, sessionCallback func(s *tracessh.Session) error) error {
	ctx, span := ns.nodeClient.Tracer.Start(
		ctx,
		"nodeClient/regularSession",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	session, err := ns.createServerSession(ctx, chanReqCallback)
	if err != nil {
		return trace.Wrap(err)
	}
	session.Stdout = ns.terminal.Stdout()
	session.Stderr = ns.terminal.Stderr()
	session.Stdin = ns.terminal.Stdin()
	return trace.Wrap(sessionCallback(session))
}

type interactiveCallback func(serverSession *tracessh.Session, shell io.ReadWriteCloser) error

func (ns *NodeSession) createServerSession(ctx context.Context, chanReqCallback tracessh.ChannelRequestCallback) (*tracessh.Session, error) {
	ctx, span := ns.nodeClient.Tracer.Start(
		ctx,
		"nodeClient/createServerSession",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	sess, err := ns.nodeClient.Client.NewSessionWithRequestCallback(ctx, chanReqCallback)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If X11 forwading is requested and the server accepts,
	// X11 channel requests from the server will be accepted.
	// Otherwise, all X11 channel requests must be rejected.
	if err := ns.handleX11Forwarding(ctx, sess); err != nil {
		return nil, trace.Wrap(err)
	}

	envs := map[string]string{}

	// pass language info into the remote session.
	langVars := []string{"LANG", "LANGUAGE"}
	for _, env := range langVars {
		if value := os.Getenv(env); value != "" {
			envs[env] = value
		}
	}
	// pass environment variables set by client
	for key, val := range ns.env {
		envs[key] = val
	}

	if err := sess.SetEnvs(ctx, envs); err != nil {
		log.WarnContext(ctx, "Failed to set environment variables", "error", err)
	}

	// if agent forwarding was requested (and we have a agent to forward),
	// forward the agent to endpoint.
	tc := ns.nodeClient.TC
	targetAgent := selectKeyAgent(tc)

	if targetAgent != nil {
		log.DebugContext(ctx, "Forwarding Selected Key Agent")
		err = agent.ForwardToAgent(ns.nodeClient.Client.Client, targetAgent)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = agent.RequestAgentForwarding(sess.Session)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return sess, nil
}

// selectKeyAgent picks the appropriate key agent for forwarding to the
// server, if any.
func selectKeyAgent(tc *TeleportClient) agent.ExtendedAgent {
	switch tc.ForwardAgent {
	case ForwardAgentYes:
		log.DebugContext(context.Background(), "Selecting system key agent")
		return connectToSSHAgent()
	case ForwardAgentLocal:
		log.DebugContext(context.Background(), "Selecting local Teleport key agent")
		return tc.localAgent.ExtendedAgent
	default:
		log.DebugContext(context.Background(), "No Key Agent selected")
		return nil
	}
}

// interactiveSession creates an interactive session on the remote node, executes
// the given callback on it, and waits for the session to end
func (ns *NodeSession) interactiveSession(ctx context.Context, mode types.SessionParticipantMode, chanReqCallback tracessh.ChannelRequestCallback, sessionCallback interactiveCallback) error {
	ctx, span := ns.nodeClient.Tracer.Start(
		ctx,
		"nodeClient/interactiveSession",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	// determine what kind of a terminal we need
	termType := os.Getenv("TERM")
	if termType == "" {
		termType = teleport.SafeTerminalType
	}
	// create the server-side session:
	sess, err := ns.createServerSession(ctx, chanReqCallback)
	if err != nil {
		return trace.Wrap(err)
	}
	// allocate terminal on the server:
	remoteTerm, err := ns.allocateTerminal(ctx, termType, sess)
	if err != nil {
		return trace.Wrap(err)
	}
	defer remoteTerm.Close()

	// call the passed callback and give them the established
	// ssh session:
	if err := sessionCallback(sess, remoteTerm); err != nil {
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
	ns.pipeInOut(ctx, remoteTerm, mode, sess)

	// wait for the session to end
	<-ns.closer.C

	// Wait for any cleanup tasks (particularly terminal reset on Windows).
	ns.closeWait.Wait()

	if ns.forceDisconnect.Load() {
		return nil
	}

	return sess.Wait()
}

// allocateTerminal creates (allocates) a server-side terminal for this session.
func (ns *NodeSession) allocateTerminal(ctx context.Context, termType string, s *tracessh.Session) (io.ReadWriteCloser, error) {
	var err error

	// read the size of the terminal window:
	width := teleport.DefaultTerminalWidth
	height := teleport.DefaultTerminalHeight
	if ns.terminal.IsAttached() {
		realWidth, realHeight, err := ns.terminal.Size()
		if err != nil {
			log.ErrorContext(ctx, "Unable to determine terminal size", "error", err)
		} else {
			width = int(realWidth)
			height = int(realHeight)
		}
	}

	// ... and request a server-side terminal of the same size:
	err = s.RequestPty(
		ctx,
		termType,
		height,
		width,
		ssh.TerminalModes{},
	)
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
		go ns.updateTerminalSize(ctx, s)
	}
	go func() {
		if _, err := io.Copy(ns.nodeClient.TC.Stderr, stderr); err != nil {
			log.DebugContext(ctx, "Error reading remote STDERR", "error", err)
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

func (ns *NodeSession) updateTerminalSize(ctx context.Context, s *tracessh.Session) {
	terminalEvents := ns.terminal.Subscribe()

	lastWidth, lastHeight, err := ns.terminal.Size()
	if err != nil {
		log.ErrorContext(ctx, "Unable to get window size", "error", err)
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
				log.WarnContext(ctx, "Unable to get window size", "error", err)
				continue
			}

			// Terminal size has not changed, don't do anything.
			if currHeight == lastHeight && currWidth == lastWidth {
				continue
			}

			// Send the "window-change" request over the channel.
			if err = s.WindowChange(ctx, int(currHeight), int(currWidth)); err != nil {
				log.WarnContext(ctx, "Unable to send window change request", "error", err)
				continue
			}

			log.DebugContext(ctx, "Updated window size from due to SIGWINCH.",
				"original_width", lastWidth,
				"original_height", lastHeight,
				"current_width", currWidth,
				"current_height", currHeight,
			)

			lastWidth, lastHeight = currWidth, currHeight

		// Extract "resize" events in the stream and store the last window size.
		case event := <-ns.nodeClient.TC.EventsChannel():
			// Only "resize" events are important to tsh, all others can be ignored.
			if event.GetType() != events.ResizeEvent {
				continue
			}

			terminalParams, err := session.UnmarshalTerminalParams(event.GetString(events.TerminalSize))
			if err != nil {
				log.WarnContext(ctx, "Unable to unmarshal terminal parameters", "error", err)
				continue
			}

			lastSize := terminalParams.Winsize()
			lastWidth = int16(lastSize.Width)
			lastHeight = int16(lastSize.Height)
			log.DebugContext(ctx, "Received window size from node in session",
				"width", lastSize.Width,
				"height", lastSize.Height,
				"session_id", event.GetString(events.SessionEventID),
			)

		// Update size of local terminal with the last size received from remote server.
		case <-tickerCh.C:
			// Get the current size of the terminal and the last size report that was
			// received.
			currWidth, currHeight, err := ns.terminal.Size()
			if err != nil {
				log.WarnContext(ctx, "Unable to get current terminal size", "error", err)
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
				log.WarnContext(ctx, "Unable to update terminal size", "error", err)
				continue
			}

			log.DebugContext(ctx, "Updated window size due to remote window change",
				"original_width", lastWidth,
				"original_height", lastHeight,
				"current_width", currWidth,
				"current_height", currHeight,
			)
		case <-ns.closer.C:
			return
		}
	}
}

// sessionWriter wraps the [tracessh.Session]
// stdout to prevent any panics that may occur
// by trying to use it before it has been initialized.
// In those cases output is written to the stdout that
// is configured for tsh so that output is not lost entirely.
type sessionWriter struct {
	tshOut  io.Writer
	session *tracessh.Session
}

func (s *sessionWriter) Write(p []byte) (int, error) {
	if s.session.Stdout != nil {
		return s.session.Stdout.Write(p)
	}

	return s.tshOut.Write(p)
}

// runShell executes user's shell on the remote node under an interactive session
func (ns *NodeSession) runShell(ctx context.Context, mode types.SessionParticipantMode, chanReqCallback tracessh.ChannelRequestCallback, beforeStart func(io.Writer), shellCallback ShellCreatedCallback) error {
	return ns.interactiveSession(ctx, mode, chanReqCallback, func(s *tracessh.Session, shell io.ReadWriteCloser) error {
		w := &sessionWriter{
			tshOut:  ns.nodeClient.TC.Stdout,
			session: s,
		}

		if beforeStart != nil {
			beforeStart(w)
		}

		// start the shell on the server:
		if err := s.Shell(ctx); err != nil {
			return trace.Wrap(err)
		}

		// call the client-supplied callback
		if shellCallback != nil {
			exit, err := shellCallback(s, ns.nodeClient.Client, shell)
			if exit {
				return trace.Wrap(err)
			}
		}
		return nil
	})
}

// runCommand executes a "exec" request either in interactive mode (with a
// TTY attached) or non-intractive mode (no TTY).
func (ns *NodeSession) runCommand(ctx context.Context, mode types.SessionParticipantMode, cmd []string, chanReqCallback tracessh.ChannelRequestCallback, shellCallback ShellCreatedCallback, interactive bool) error {
	ctx, span := ns.nodeClient.Tracer.Start(
		ctx,
		"nodeClient/runCommand",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	// Start a interactive session ("exec" request with a TTY).
	//
	// Note that because a TTY was allocated, the terminal is in raw mode and any
	// keyboard based signals will be propogated to the TTY on the server which is
	// where all signal handling will occur.
	if interactive {
		return ns.interactiveSession(ctx, mode, chanReqCallback, func(s *tracessh.Session, term io.ReadWriteCloser) error {
			err := s.Start(ctx, strings.Join(cmd, " "))
			if err != nil {
				return trace.Wrap(err)
			}
			if shellCallback != nil {
				exit, err := shellCallback(s, ns.NodeClient().Client, term)
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
	return ns.regularSession(ctx, chanReqCallback, func(s *tracessh.Session) error {
		errCh := make(chan error, 1)
		go func() {
			errCh <- s.Run(ctx, strings.Join(cmd, " "))
		}()

		select {
		// Run returned a result, return that back to the caller.
		case err := <-errCh:
			return trace.Wrap(err)
		// The passed in context timed out. This is often due to the user hitting
		// Ctrl-C.
		case <-ctx.Done():
			if err := s.Close(); err != nil {
				log.DebugContext(ctx, "Unable to close SSH channel", "error", err)
			}
			if err := ns.NodeClient().Client.Close(); err != nil {
				log.DebugContext(ctx, "Unable to close SSH client", "error", err)
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
					log.ErrorContext(context.Background(), "Failed to forward ctrl+c", "error", err)
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
					log.ErrorContext(context.Background(), "Failed to forward ctrl+z", "error", err)
				}
			}
		}
	}()
}

func handleNonPeerControls(mode types.SessionParticipantMode, term *terminal.Terminal, forceTerminate func()) {
	for {
		buf := make([]byte, 1)
		_, err := term.Stdin().Read(buf)
		if errors.Is(err, io.EOF) {
			return
		}

		// Ctrl-C
		if buf[0] == '\x03' {
			fmt.Print("\n\rLeft session\n\r")
			return
		}

		// t
		if buf[0] == 't' && mode == types.SessionModeratorMode {
			fmt.Print("\n\rForcefully terminating session\n\r")
			forceTerminate()
			break
		}
	}
}

// handlePeerControls streams the terminal input to the remote shell's standard input.
// Escape sequences for stopping the stream on the client side are supported via `escape.NewReader`.
//
// If the `forceDisconnect` boolean is true upon return, the session must be instantly terminated without
// waiting for any remote task to finish.
func handlePeerControls(term *terminal.Terminal, enableEscapeSequences bool, remoteStdin io.Writer) (forceDisconnect bool) {
	stdin := term.Stdin()
	if enableEscapeSequences {
		// escape.NewReader is used to enable manual disconnect sequences as those supported
		// by tsh. These can be used to force a client disconnect since CTRL-C is merely passed
		// to the other end and not interpreted as an exit request locally
		stdin = escape.NewReader(stdin, term.Stderr(), func(err error) {
			log.DebugContext(context.Background(), "escape.NewReader error", "error", err)

			switch {
			case errors.Is(err, escape.ErrDisconnect):
				fmt.Fprint(term.Stderr(), "\r\nDisconnected\r\n")
			case errors.Is(err, escape.ErrTooMuchBufferedData):
				fmt.Fprint(term.Stderr(), "\r\nRemote peer may be unreachable, check your connectivity\r\n")
			default:
				fmt.Fprintf(term.Stderr(), "\r\nunknown error: %v\r\n", err.Error())
			}
		})
	}

	_, err := io.Copy(remoteStdin, stdin)
	if err != nil {
		log.DebugContext(context.Background(), "Error copying data to remote peer", "error", err)
		fmt.Fprint(term.Stderr(), "\r\nError copying data to remote peer\r\n")
		forceDisconnect = true
	}

	return forceDisconnect
}

// pipeInOut launches two goroutines: one to pipe the local input into the remote shell,
// and another to pipe the output of the remote shell into the local output
func (ns *NodeSession) pipeInOut(ctx context.Context, shell io.ReadWriteCloser, mode types.SessionParticipantMode, sess *tracessh.Session) {
	// copy from the remote shell to the local output
	go func() {
		defer ns.closer.Close()
		_, err := io.Copy(ns.terminal.Stdout(), shell)
		if err != nil && !utils.IsOKNetworkError(err) {
			log.ErrorContext(ctx, "Failed copying data to session", "error", err)
		}
	}()

	switch mode {
	case types.SessionObserverMode, types.SessionModeratorMode:
		go func() {
			defer ns.closer.Close()

			handleNonPeerControls(mode, ns.terminal, func() {
				_, err := sess.SendRequest(ctx, teleport.ForceTerminateRequest, true, nil)
				if err != nil {
					fmt.Printf("\n\rError while sending force termination request: %v\n\r", err.Error())
				}
			})

			// Force disconnect the session. We want to release the local terminal
			// connected to the session rather than wait for the session to end.
			ns.forceDisconnect.Store(true)
		}()
	case types.SessionPeerMode:
		// copy from the local input to the remote shell:
		go func() {
			if handlePeerControls(ns.terminal, ns.enableEscapeSequences, shell) {
				ns.forceDisconnect.Store(true)
			}

			ns.closer.Close()
		}()
	}
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
