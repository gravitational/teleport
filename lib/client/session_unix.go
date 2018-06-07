// +build !windows

package client

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/moby/moby/pkg/term"

	log "github.com/sirupsen/logrus"
)

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
			log.Debugf("Recevied window size %v from node in session.\n", lastSize, event.GetString(events.SessionEventID))

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
				log.Warnf("Unable to update terminal size: %v.\n", err)
				continue
			}

			// This is what we use to resize the physical terminal window itself.
			os.Stdout.Write([]byte(fmt.Sprintf("\x1b[8;%d;%dt", lastSize.Height, lastSize.Width)))

			log.Debugf("Updated window size from to %v due to remote window change.", currSize, lastSize)
		case <-ns.closer.C:
			return
		}
	}
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
