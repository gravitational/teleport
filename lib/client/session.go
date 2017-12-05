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
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/moby/moby/pkg/term"

	log "github.com/sirupsen/logrus"
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
}

// newSession creates a new Teleport session with the given remote node
// if 'joinSessin' is given, the session will join the existing session
// of another user
func newSession(client *NodeClient,
	joinSession *session.Session,
	env map[string]string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer) (*NodeSession, error) {

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
		env:        env,
		nodeClient: client,
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		namespace:  client.Namespace,
		closer:     utils.NewCloseBroadcaster(),
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
			sid = string(session.NewID())
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
	ns.env["TERM"] = termType
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
	defer remoteTerm.Close()

	// call the passed callback and give them the established
	// ssh session:
	if err := callback(sess, remoteTerm); err != nil {
		return trace.Wrap(err)
	}

	// Catch term signals, but only if we're attached to a real terminal
	if ns.isTerminalAttached() {
		ns.watchSignals(remoteTerm)
	}

	// start piping input into the remote shell and pipe the output from
	// the remote shell into stdout:
	ns.pipeInOut(remoteTerm)

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
		io.Copy(os.Stderr, stderr)
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
	// sibscribe for "terminal resized" signal:
	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGWINCH)
	currentSize, _ := term.GetWinsize(0)

	// start the timer which asks for server-side window size changes:
	siteClient, err := ns.nodeClient.Proxy.ConnectToSite(context.TODO(), true)
	if err != nil {
		log.Error(err)
		return
	}
	tick := time.NewTicker(defaults.SessionRefreshPeriod)
	defer tick.Stop()

	var prevSess *session.Session
	for {
		select {
		// our own terminal window got resized:
		case sig := <-sigC:
			if sig == nil {
				return
			}
			// get the size:
			winSize, err := term.GetWinsize(0)
			if err != nil {
				log.Warnf("[CLIENT] Error getting size: %s", err)
				break
			}
			// it's the result of our own size change (see below)
			if winSize.Height == currentSize.Height && winSize.Width == currentSize.Width {
				continue
			}
			// send the new window size to the server
			_, err = s.SendRequest(
				sshutils.WindowChangeRequest, false,
				ssh.Marshal(sshutils.WinChangeReqParams{
					W: uint32(winSize.Width),
					H: uint32(winSize.Height),
				}))
			if err != nil {
				log.Warnf("[CLIENT] failed to send window change reqest: %v", err)
			}
		case <-tick.C:
			sess, err := siteClient.GetSession(ns.namespace, ns.id)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.Error(trace.DebugReport(err))
				}
				continue
			}
			// no previous session
			if prevSess == nil || sess == nil {
				prevSess = sess
				continue
			}
			// nothing changed
			if prevSess.TerminalParams.W == sess.TerminalParams.W && prevSess.TerminalParams.H == sess.TerminalParams.H {
				continue
			}
			log.Infof("[CLIENT] updating the session %v with %d parties", sess.ID, len(sess.Parties))

			newSize := sess.TerminalParams.Winsize()
			currentSize, err = term.GetWinsize(0)
			if err != nil {
				log.Error(err)
			}
			if currentSize.Width != newSize.Width || currentSize.Height != newSize.Height {
				// ok, something have changed, let's resize to the new parameters
				err = term.SetWinsize(0, newSize)
				if err != nil {
					log.Error(err)
				}
				os.Stdout.Write([]byte(fmt.Sprintf("\x1b[8;%d;%dt", newSize.Height, newSize.Width)))
			}
			prevSess = sess
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

// runCommand executes a given command either in interactive (with terminal attached)
// or non-intractive mode
func (ns *NodeSession) runCommand(cmd []string, callback ShellCreatedCallback, interactive bool) error {
	// stdin is not a terminal? refuse to allocate terminal on the server and go back
	// to "non-interactive":
	if interactive && ns.stdin == os.Stdin && !term.IsTerminal(os.Stdin.Fd()) {
		interactive = false
		fmt.Fprintf(os.Stderr, "TTY will not be allocated on the server because stdin is not a terminal\n")
	}

	// interactive session:
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
	// non-interactive session:
	return ns.regularSession(func(s *ssh.Session) error {
		return s.Run(strings.Join(cmd, " "))
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
func (ns *NodeSession) pipeInOut(shell io.ReadWriteCloser) {
	// copy from the remote shell to the local output
	go func() {
		defer ns.closer.Close()
		_, err := io.Copy(ns.stdout, shell)
		if err != nil {
			log.Errorf(err.Error())
		}
	}()
	// copy from the local input to the remote shell:
	go func() {
		defer ns.closer.Close()
		buf := make([]byte, 128)
		for {
			n, err := ns.stdin.Read(buf)
			if err != nil {
				fmt.Fprintln(ns.stderr, trace.Wrap(err))
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
	return nil
}
