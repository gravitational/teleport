// +build windows

/*
Copyright 2018 Gravitational, Inc.

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
	"golang.org/x/sys/windows"

	"github.com/Azure/go-ansiterm/winterm"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client/escape"
	"github.com/gravitational/teleport/lib/client/tncon"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/ActiveState/termtest/conpty"
	"github.com/moby/term"
)

// NodeSession is a bare minimum implementation to get Windows to compile.
// This sits behind a build flag because github.com/docker/docker/pkg/term
// on Windows does not support "SetWinsize". Because tsh on Windows does not
// support "tsh ssh" this code will never be called.
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

	conpty *conpty.ConPty
}

// requiredSyscalls lists all syscalls required to initialize the ConPTY.
func requiredSyscalls() []string {
	return []string{
		"CreatePseudoConsole",
		"ResizePseudoConsole",
		"ClosePseudoConsole",
		"InitializeProcThreadAttributeList",
		"UpdateProcThreadAttribute",
		"DeleteProcThreadAttributeList",
	}
}

// ensureSyscallsExist validates that all listed Windows syscalls are present.
func ensureSyscallsExist() error {
	var missing []string

	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	for _, name := range requiredSyscalls() {
		err := kernel32.NewProc(name).Find()
		if err != nil {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf(`syscalls required for tsh.exe ssh on Windows could not be found.
Ensure Windows 10 1809 or later is installed.

Missing syscalls: %s`, missing)
	} else {
		return nil
	}
}

// initTerminal configures the terminal for raw, VT compatible input and output.
// The returned function should be called before program exit to ensure the
// terminal is reset, otherwise it will be left in a broken state.
func initTerminal() (func(), error) {
	stdoutFd := int(syscall.Stdout)
	stdinFd := int(syscall.Stdin)

	oldOutMode, err := winterm.GetConsoleMode(uintptr(stdoutFd))
	if err != nil {
		return func() {}, fmt.Errorf("failed to retrieve stdout mode: %w", err)
	}

	oldInMode, err := winterm.GetConsoleMode(uintptr(stdinFd))
	if err != nil {
		return func() {}, fmt.Errorf("failed to retrieve stdout mode: %w", err)
	}

	newOutMode := oldOutMode | winterm.ENABLE_VIRTUAL_TERMINAL_PROCESSING | winterm.DISABLE_NEWLINE_AUTO_RETURN

	err = winterm.SetConsoleMode(uintptr(stdoutFd), newOutMode)
	if err != nil {
		return func() {}, fmt.Errorf("failed to set stdout mode: %w", err)
	}

	newInMode := oldInMode
	newInMode &^= winterm.ENABLE_ECHO_INPUT
	newInMode &^= winterm.ENABLE_LINE_INPUT
	newInMode &^= winterm.ENABLE_MOUSE_INPUT
	newInMode &^= winterm.ENABLE_WINDOW_INPUT
	newInMode &^= winterm.ENABLE_PROCESSED_INPUT

	newInMode |= winterm.ENABLE_EXTENDED_FLAGS
	newInMode |= winterm.ENABLE_INSERT_MODE
	newInMode |= winterm.ENABLE_QUICK_EDIT_MODE
	newInMode |= winterm.ENABLE_VIRTUAL_TERMINAL_INPUT

	err = winterm.SetConsoleMode(uintptr(stdinFd), newInMode)
	if err != nil {
		return func() {}, fmt.Errorf("failed to set stdin mode: %w", err)
	}

	return func() {
		err = winterm.SetConsoleMode(uintptr(stdoutFd), oldOutMode)
		if err != nil {
			log.Errorf("Failed to reset output terminal mode to %d: %v\n", oldOutMode, err)
		}

		err = winterm.SetConsoleMode(uintptr(stdinFd), oldInMode)
		if err != nil {
			log.Errorf("Failed to reset output terminal mode to %d: %v\n", oldInMode, err)
		}
	}, nil
}

// resizeTerminal attempts to resize compatible Windows terminals. This is
// known to usually work on cmd.exe and PowerShell (with occasional failures)
// but is not currently supported by the new Windows Terminal app.
func resizeTerminal(rows int16, cols int16) error {
	if rows < 1 || cols < 1 {
		return fmt.Errorf("cannot shrink terminal below 1x1: rows=%d, cols=%d", rows, cols)
	}

	stdoutFd := uintptr(int(syscall.Stdout))

	// Hack: the buffer can't be smaller than the window, and the window can't
	// be bigger than the buffer otherwise we'll just get an inscrutible
	// "The parameter is incorrect" error. As a workaround, first resize the
	// window to the minimum possible size:
	err := winterm.SetConsoleWindowInfo(stdoutFd, true, winterm.SMALL_RECT{
		Left:   0,
		Top:    0,
		Right:  1,
		Bottom: 1,
	})
	if err != nil {
		return fmt.Errorf("shrinking the console window: %w", err)
	}

	// ... then we can freely set the buffer:
	err = winterm.SetConsoleScreenBufferSize(stdoutFd, winterm.COORD{
		X: cols,
		Y: rows,
	})
	if err != nil {
		return fmt.Errorf("setting screen buffer size: %w", err)
	}

	// ... and finally we can set the window's size to its desired value.
	err = winterm.SetConsoleWindowInfo(stdoutFd, true, winterm.SMALL_RECT{
		Left:   0,
		Top:    0,
		Right:  cols - 1,
		Bottom: rows - 1,
	})
	if err != nil {
		return fmt.Errorf("setting console window info: %w", err)
	}

	return nil
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
	// The conpty library blindly attempts to load an execute Windows syscalls
	// that are only present in Windows 10 v1809 or later, and will cause a
	// panic if they do not exist. Check first to ensure older Windows releases
	// exit with an informative error message.
	err := ensureSyscallsExist()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cpty, err := conpty.New(teleport.DefaultTerminalWidth, teleport.DefaultTerminalHeight)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if stdin == nil {
		stdin = cpty.InPipe()
	}
	if stdout == nil {
		stdin = cpty.OutPipe()
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	if env == nil {
		env = make(map[string]string)
	}

	ns := &NodeSession{
		env:                   env,
		nodeClient:            client,
		stdin:                 stdin,
		stdout:                stdout,
		stderr:                stderr,
		namespace:             client.Namespace,
		closer:                utils.NewCloseBroadcaster(),
		enableEscapeSequences: enableEscapeSequences,
		conpty:                cpty,
	}
	// if we're joining an existing session, we need to assume that session's
	// existing/current terminal size:
	if joinSession != nil {
		ns.id = joinSession.ID
		ns.namespace = joinSession.Namespace
		tsize := joinSession.TerminalParams.Winsize()
		if ns.isTerminalAttached() {
			err = ns.conpty.Resize(tsize.Width, tsize.Height)
			if err != nil {
				log.Errorf("Failed to resize ConPTY: %v", err)
			}

			// Note: This works only on cmd.exe and PowerShell. There is
			// currently no way to programmatically resize the new Terminal app.
			resizeTerminal(int16(tsize.Height), int16(tsize.Width))
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

	// Close the ConPty when finished.
	go func() {
		<-ns.closer.C
		ns.conpty.Close()
	}()

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
		deinit, err := initTerminal()
		if err != nil {
			return trace.Wrap(err)
		}

		go func() {
			<-ns.closer.C
			deinit()
		}()
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
		realSize, err := term.GetWinsize(uintptr(syscall.Stdout))
		if err != nil {
			log.Error(err)
		} else {
			tsize = realSize
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
	// NOTE: While we do receive terminal resize events, they are only reliable
	// under certain Windows terminals. This implementation currently polls for
	// size changes at a `defaults.TerminalResizePeriod` which is slow but
	// reliable.

	// lastSize is the last known client-side size.
	lastSize, err := term.GetWinsize(uintptr(int(syscall.Stdout)))
	if err != nil {
		log.Errorf("Unable to get window size: %v", err)
		return
	}

	// desiredSize is a (possibly nil) size request from the remote channel.
	var desiredSize *term.Winsize

	// Sync the local terminal with size received from the remote server every
	// two seconds. If we try and do it live, synchronization jitters occur.
	tickerCh := time.NewTicker(defaults.TerminalResizePeriod)
	defer tickerCh.Stop()

	for {
		select {
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

			desiredSize = terminalParams.Winsize()
			log.Debugf("Recevied window size %v from node in session %v.", lastSize, event.GetString(events.SessionEventID))

		// Update size of local terminal with the last size received from remote server.
		case <-tickerCh.C:
			// Get the current size of the terminal and the last size report that was
			// received.
			currSize, err := term.GetWinsize(uintptr(int(syscall.Stdout)))
			if err != nil {
				log.Warnf("Unable to get current terminal size: %v.", err)
				continue
			}

			if desiredSize != nil && (desiredSize.Width != currSize.Width || desiredSize.Height != currSize.Height) {
				// Remote request to resize the terminal.

				// Resize the underlying ConPTY.
				err = ns.conpty.Resize(desiredSize.Width, desiredSize.Height)
				if err != nil {
					log.Warnf("Unable to update ConPTY size: %v.", err)
					continue
				}

				// Attempt to resize the terminal window itself.
				// Note: This only works on cmd.exe and PowerShell, but is a
				// no-op on the new Windows Terminal app.
				// Note: cmd.exe and PowerShell also support the `ESC[8;...`
				// sequence, however it doesn't play nice with our ConPTY pipe.
				resizeTerminal(int16(desiredSize.Height), int16(desiredSize.Width))

				log.Debugf("Updated window size from %v to %v due to remote window change.", currSize, lastSize)

				// Clear the desired size.
				desiredSize = nil
			} else if currSize.Width != lastSize.Width || currSize.Height != lastSize.Height {
				// Local terminal size has changed. Resize the pty and notify
				// the remote.

				lastSize = currSize

				// Resize the underlying ConPTY.
				err = ns.conpty.Resize(currSize.Width, currSize.Height)
				if err != nil {
					log.Warnf("Unable to update ConPTY size: %v.", err)
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

				log.Debugf("Updated remote session about new local windows size %v", currSize)

				continue
			}

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

	// Note: On Windows, ReadConsoleInput captures most signal sequences
	// (including Ctrl-Z and Ctrl-C) and emits them directly as VT sequences
	// which we can pipe to the remote.

	// Catch the interrupt signals. Note that this signal is never triggered by
	// Ctrl-C in Windows when terminal input processing is disabled. We'll keep
	// this handler to respond as expected if signalled externally.
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

	// Note: Windows does not support SIGTSTP (Ctrl-Z), but our use of
	// ReadConsoleInput ensures the key combo is sent anyway.
}

// pipeInOut launches two goroutines: one to pipe the local input into the remote shell,
// and another to pipe the output of the remote shell into the local output
func (ns *NodeSession) pipeInOut(shell io.ReadWriteCloser) {
	// Read raw console events via the Windows API.
	go tncon.ReadInputContinuous()

	// copy from the remote shell to the local output
	go func() {
		defer ns.closer.Close()
		_, err := io.Copy(ns.stdout, shell)
		if err != nil {
			log.Errorf(err.Error())
		}
	}()

	pipeRead, pipeWrite := io.Pipe()

	// Subscribe to console events and pass them along to the pipe.
	go func() {
		defer ns.closer.Close()

		events := tncon.Subscribe()
		for event := range events {
			switch ev := event.(type) {
			case tncon.SequenceEvent:
				if len(ev.Sequence) > 0 {
					_, err := pipeWrite.Write(ev.Sequence)
					if err != nil {
						ns.ExitMsg = err.Error()
						return
					}
				}
			}
		}
	}()

	// copy from the local input to the remote shell:
	go func() {
		defer ns.closer.Close()
		buf := make([]byte, 128)

		var stdin io.Reader
		if ns.isTerminalAttached() && ns.enableEscapeSequences {
			stdin = escape.NewReader(pipeRead, ns.stderr, func(err error) {
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
		} else {
			stdin = pipeRead
		}
		for {
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
	}()
}

func (ns *NodeSession) Close() error {
	if ns.closer != nil {
		ns.closer.Close()
	}
	return nil
}
