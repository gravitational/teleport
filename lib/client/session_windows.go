package client

import (
  "io"
  "os"
  "os/signal"
  "syscall"

  "golang.org/x/crypto/ssh"

  log "github.com/sirupsen/logrus"
)

func (ns *NodeSession) updateTerminalSize(s *ssh.Session) {
  return
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
}
