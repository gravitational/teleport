// +build !windows

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

	"github.com/moby/term"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client/escape"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
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

	// Standard input/outputs for this session
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	// closer is used to simultaneously close all goroutines created by
	// this session. It's also used to wait for everyone to close
	closer *utils.CloseBroadcaster

	ExitMsg string

	enableEscapeSequences bool
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

	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	if env == nil {
		env = make(map[string]string)
	}

	var err error
	ns := &NodeSession{
		env:                   env,
		nodeClient:            client,
		stdin:                 stdin,
		stdout:                stdout,
		stderr:                stderr,
		namespace:             client.Namespace,
		closer:                utils.NewCloseBroadcaster(),
		enableEscapeSequences: enableEscapeSequences,
	}
	// if we're joining an existing session, we need to assume that session's
	// existing/current terminal size:
	if joinSession != nil {
		ns.id = joinSession.ID
		ns.namespace = joinSession.Namespace
		tsize := joinSession.TerminalParams.Winsize()
		if ns.isTerminalAttached() {
			err = term.SetWinsize(0, tsize)
			if err != nil {
				log.Error(err)
			}
			os.Stdout.Write([]byte(fmt.Sprintf("\x1b[8;%d;%dt", tsize.Height, tsize.Width)))
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
	return ns, nil
}

func (ns *NodeSession) NodeClient() *NodeClient {
	return ns.nodeClient
}

func (ns *NodeSession) regularSession(callback func(s *ssh.Session) error) error {
	session, err := ns.createServerSession()
	if err != nil {
		return trace.Wrap(err)
	}
	session.Stdout = ns.stdout
	session.Stderr = ns.stderr
	session.Stdin = ns.stdin
	return trace.Wrap(callback(session))
}

type interactiveCallback func(serverSession *ssh.Session, shell io.ReadWriteCloser) error

func (ns *NodeSession) createServerSession() (*ssh.Session, error) {
	sess, err := ns.nodeClient.Client.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
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
	if tc.ForwardAgent && tc.localAgent.Agent != nil {
		err = agent.ForwardToAgent(ns.nodeClient.Client, tc.localAgent.Agent)
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

// interactiveSession creates an interactive session on the remote node, executes
// the given callback on it, and waits for the session to end
func (ns *NodeSession) interactiveSession(callback interactiveCallback) error {
	// determine what kind of a terminal we need
	termType := os.Getenv("TERM")
	if termType == "" {
		termType = teleport.SafeTerminalType
	}
	// create the server-side session:
	sess, err := ns.createServerSession()
	if err != nil {
		return trace.Wrap(err)
	}
	// allocate terminal on the server:
	remoteTerm, err := ns.allocateTerminal(termType, sess)
	if err != nil {
		return trace.Wrap(err)
	}

	// call the passed callback and give them the established
	// ssh session:
	if err := callback(sess, remoteTerm); err != nil {
		remoteTerm.Close()
		return trace.Wrap(err)
	}

	// Catch term signals, but only if we're attached to a real terminal
	if ns.isTerminalAttached() {
		ns.watchSignals(remoteTerm)
	}

	// start piping input into the remote shell and pipe the output from
	// the remote shell into stdout:
	// Note, pipeInOut takes ownership of remoteTerm and will close it
	// upon completion
	var wg sync.WaitGroup
	ns.pipeInOut(remoteTerm, &wg)

	// switch the terminal to raw mode (and switch back on exit!)
	if ns.isTerminalAttached() {
		ts, err := term.SetRawTerminal(0)
		if err != nil {
			log.Warn(err)
		} else {
			defer term.RestoreTerminal(0, ts)
		}
	}
	// wait for the session to end
	<-ns.closer.C
	wg.Wait()
	return nil
}

// allocateTerminal creates (allocates) a server-side terminal for this session.
func (ns *NodeSession) allocateTerminal(termType string, s *ssh.Session) (io.ReadWriteCloser, error) {
	var err error
	// read the size of the terminal window:
	tsize := &term.Winsize{
		Width:  teleport.DefaultTerminalWidth,
		Height: teleport.DefaultTerminalHeight,
	}
	if ns.isTerminalAttached() {
		tsize, err = term.GetWinsize(0)
		if err != nil {
			log.Error(err)
		}
	}
	// ... and request a server-side terminal of the same size:
	err = s.RequestPty(termType,
		int(tsize.Height),
		int(tsize.Width),
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
	if ns.isTerminalAttached() {
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
	// SIGWINCH is sent to the process when the window size of the terminal has
	// changed.
	sigwinchCh := make(chan os.Signal, 1)
	signal.Notify(sigwinchCh, syscall.SIGWINCH)

	lastSize, err := term.GetWinsize(0)
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
		case sigwinch := <-sigwinchCh:
			if sigwinch == nil {
				return
			}

			currSize, err := term.GetWinsize(0)
			if err != nil {
				log.Warnf("Unable to get window size: %v.", err)
				continue
			}

			// Terminal size has not changed, don't do anything.
			if currSize.Height == lastSize.Height && currSize.Width == lastSize.Width {
				continue
			}

			// Send the "window-change" request over the channel.
			_, err = s.SendRequest(
				sshutils.WindowChangeRequest,
				false,
				ssh.Marshal(sshutils.WinChangeReqParams{
					W: uint32(currSize.Width),
					H: uint32(currSize.Height),
				}))
			if err != nil {
				log.Warnf("Unable to send %v reqest: %v.", sshutils.WindowChangeRequest, err)
				continue
			}

			log.Debugf("Updated window size from %v to %v due to SIGWINCH.", lastSize, currSize)

			lastSize = currSize

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

			lastSize = terminalParams.Winsize()
			log.Debugf("Recevied window size %v from node in session %v.", lastSize, event.GetString(events.SessionEventID))

		// Update size of local terminal with the last size received from remote server.
		case <-tickerCh.C:
			// Get the current size of the terminal and the last size report that was
			// received.
			currSize, err := term.GetWinsize(0)
			if err != nil {
				log.Warnf("Unable to get current terminal size: %v.", err)
				continue
			}

			// Terminal size has not changed, don't do anything.
			if currSize.Width == lastSize.Width && currSize.Height == lastSize.Height {
				continue
			}

			// This changes the size of the local PTY. This will re-draw what's within
			// the window.
			err = term.SetWinsize(0, lastSize)
			if err != nil {
				log.Warnf("Unable to update terminal size: %v.", err)
				continue
			}

			// This is what we use to resize the physical terminal window itself.
			os.Stdout.Write([]byte(fmt.Sprintf("\x1b[8;%d;%dt", lastSize.Height, lastSize.Width)))

			log.Debugf("Updated window size from %v to %v due to remote window change.", currSize, lastSize)
		case <-ns.closer.C:
			return
		}
	}
}

// isTerminalAttached returns true when this session is be controlled by
// a real terminal.
// It will return False for sessions initiated by the Web client or
// for non-interactive sessions (commands)
func (ns *NodeSession) isTerminalAttached() bool {
	return ns.stdin == os.Stdin && term.IsTerminal(os.Stdin.Fd())
}

// runShell executes user's shell on the remote node under an interactive session
func (ns *NodeSession) runShell(callback ShellCreatedCallback) error {
	return ns.interactiveSession(func(s *ssh.Session, shell io.ReadWriteCloser) error {
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
	if interactive && ns.stdin == os.Stdin && !term.IsTerminal(os.Stdin.Fd()) {
		interactive = false
		fmt.Fprintf(os.Stderr, "TTY will not be allocated on the server because stdin is not a terminal\n")
	}

	// Start a interactive session ("exec" request with a TTY).
	//
	// Note that because a TTY was allocated, the terminal is in raw mode and any
	// keyboard based signals will be propogated to the TTY on the server which is
	// where all signal handling will occur.
	if interactive {
		return ns.interactiveSession(func(s *ssh.Session, term io.ReadWriteCloser) error {
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
	return ns.regularSession(func(s *ssh.Session) error {
		var err error

		runContext, cancel := context.WithCancel(context.Background())
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
	exitSignals := make(chan os.Signal, 1)
	// catch SIGTERM
	signal.Notify(exitSignals, syscall.SIGTERM)
	go func() {
		defer ns.closer.Close()
		<-exitSignals
	}()
	// Catch Ctrl-C signal
	ctrlCSignal := make(chan os.Signal, 1)
	signal.Notify(ctrlCSignal, syscall.SIGINT)
	go func() {
		for {
			<-ctrlCSignal
			_, err := shell.Write([]byte{3})
			if err != nil {
				log.Errorf(err.Error())
			}
		}
	}()
	// Catch Ctrl-Z signal
	ctrlZSignal := make(chan os.Signal, 1)
	signal.Notify(ctrlZSignal, syscall.SIGTSTP)
	go func() {
		for {
			<-ctrlZSignal
			_, err := shell.Write([]byte{26})
			if err != nil {
				log.Errorf(err.Error())
			}
		}
	}()
}

// pipeInOut launches two goroutines: one to pipe the local input into the remote shell,
// and another to pipe the output of the remote shell into the local output
// func (ns *NodeSession) pipeInOut(shell io.ReadWriteCloser, wg *sync.WaitGroup) {
func (ns *NodeSession) pipeInOut(shell io.ReadWriteCloser, wg *sync.WaitGroup) {
	// copy from the remote shell to the local output
	wg.Add(2)
	go func() {
		defer ns.closer.Close()
		defer wg.Done()
		_, err := io.Copy(ns.stdout, shell)
		if err != nil {
			log.Error("Error copying from shell:", err.Error())
		}
	}()
	// copy from the local input to the remote shell:
	go func() {
		defer ns.closer.Close()
		defer shell.Close()
		defer wg.Done()
		buf := make([]byte, 128)

		stdin := ns.stdin
		if ns.isTerminalAttached() && ns.enableEscapeSequences {
			stdin = escape.NewReader(stdin, ns.stderr, func(err error) {
				switch err {
				case escape.ErrDisconnect:
					fmt.Fprintf(ns.stderr, "\r\n%v\r\n", err)
				case escape.ErrTooMuchBufferedData:
					fmt.Fprintf(ns.stderr, "\r\nerror: %v\r\nremote peer may be unreachable, check your connectivity\r\n", trace.Wrap(err))
				default:
					fmt.Fprintf(ns.stderr, "\r\nerror: %v\r\n", trace.Wrap(err))
				}
				ns.closer.Close()
			})
		}
		for {
			select {
			case <-ns.closer.C:
				return
			default:
				n, err := stdin.Read(buf)
				if err != nil {
					fmt.Fprintf(ns.stderr, "\r\n%v\r\n", trace.Wrap(err))
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
		}
	}()
}

func (ns *NodeSession) Close() error {
	return ns.closer.Close()
}
