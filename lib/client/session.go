package client

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/term"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	log "github.com/Sirupsen/logrus"
)

type NodeSession struct {
	// id is the Teleport session ID
	id session.ID

	// env is the environment variables that need to be created
	// on the server for this session
	env map[string]string

	// attachedTerm is set to true when this session is be controlled by
	// a real terminal.
	// This will be set to False for sessions initiated by the Web client or
	// for non-interactive sessions (commands)
	attachedTerm bool

	// terminalSize is the inital size of the terminal. It only has meaning
	// when the session is interactive
	terminalSize *term.Winsize

	// serverSession is the server-side SSH session
	serverSession *ssh.Session

	// nodeClient is the parent of this session: the client connected to an
	// SSH node
	nodeClient *NodeClient
}

func newSession(client *NodeClient,
	joinSession *session.Session,
	env map[string]string,
	attachedTerm bool) (*NodeSession, error) {

	var err error
	ns := &NodeSession{
		attachedTerm: attachedTerm,
		env:          env,
		nodeClient:   client,
		terminalSize: &term.Winsize{Width: 80, Height: 25},
	}

	// read the size of the terminal window:
	if attachedTerm {
		ns.terminalSize, err = term.GetWinsize(0)
		if err != nil {
			log.Error(err)
		}
		state, err := term.SetRawTerminal(0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer term.RestoreTerminal(0, state)
	}

	// if we're joining an existing session, we need to assume that session's
	// existing/current terminal size:
	if joinSession != nil {
		ns.id = joinSession.ID
		ns.terminalSize = joinSession.TerminalParams.Winsize()
		if attachedTerm {
			err = term.SetWinsize(0, ns.terminalSize)
			if err != nil {
				log.Error(err)
			}
			os.Stdout.Write([]byte(fmt.Sprintf("\x1b[8;%d;%dt", ns.terminalSize.Height, ns.terminalSize.Width)))
		}
		// new session!
	} else {
		ns.id = session.NewID()
	}
	if ns.env == nil {
		ns.env = make(map[string]string)
	}
	ns.env[sshutils.SessionEnvVar] = string(ns.id)

	// create the server-side session:
	ns.serverSession, err = client.Client.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// pass language info into the remote session.
	evarsToPass := []string{"LANG", "LANGUAGE"}
	for _, evar := range evarsToPass {
		if value := os.Getenv(evar); value != "" {
			err = ns.serverSession.Setenv(evar, value)
			if err != nil {
				log.Warn(err)
			}
		}
	}
	// pass environment variables set by client
	for key, val := range env {
		err = ns.serverSession.Setenv(key, val)
		if err != nil {
			log.Warn(err)
		}
	}
	return ns, nil
}

// allocateTerminal creates (allocates) a server-side terminal for a given session.
func (ns *NodeSession) allocateTerminal() (io.ReadWriteCloser, error) {
	err := ns.serverSession.RequestPty("xterm",
		int(ns.terminalSize.Height),
		int(ns.terminalSize.Width),
		ssh.TerminalModes{})

	if err != nil {
		return nil, trace.Wrap(err)
	}
	writer, err := ns.serverSession.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reader, err := ns.serverSession.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stderr, err := ns.serverSession.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	closer := utils.NewCloseBroadcaster()
	if ns.attachedTerm {
		go ns.updateTerminalSize(closer)
	}
	go func() {
		io.Copy(os.Stderr, stderr)
	}()
	return utils.NewPipeNetConn(
		reader,
		writer,
		utils.MultiCloser(writer, ns.serverSession, closer),
		&net.IPAddr{},
		&net.IPAddr{},
	), nil
}

func (ns *NodeSession) updateTerminalSize(closer *utils.CloseBroadcaster) {
	// sibscribe for "terminal resized" signal:
	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGWINCH)
	currentSize, _ := term.GetWinsize(0)

	// start the timer which asks for server-side window size changes:
	siteClient, err := ns.nodeClient.Proxy.ConnectToSite()
	if err != nil {
		log.Error(err)
	}
	tick := time.NewTicker(defaults.SessionRefreshPeriod)
	defer tick.Stop()

	var prevSess *session.Session
	for {
		select {
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
			_, err = ns.serverSession.SendRequest(
				sshutils.WindowChangeReq, false,
				ssh.Marshal(sshutils.WinChangeReqParams{
					W: uint32(winSize.Width),
					H: uint32(winSize.Height),
				}))
			if err != nil {
				log.Warnf("[CLIENT] failed to send window change reqest: %v", err)
			}
		case <-tick.C:
			sess, err := siteClient.GetSession(ns.id)
			if err != nil {
				log.Error(err)
				continue
			}
			// no previous session
			if prevSess == nil || sess == nil {
				prevSess = sess
				continue
			}
			log.Infof("[CLIENT] updating the session %v with %d parties", sess.ID, len(sess.Parties))
			// nothing changed
			if prevSess.TerminalParams.W == sess.TerminalParams.W && prevSess.TerminalParams.H == sess.TerminalParams.H {
				continue
			}

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
		case <-closer.C:
			return
		}
	}
}
