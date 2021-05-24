package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTSH,
})

func main() {
	ctx := setupSignalHandlers()

	var debug bool
	app := kingpin.New("term", "Term is a teleport terminal")
	app.Flag("debug", "Turn on debugging level").Short('d').BoolVar(&debug)

	webC := app.Command("web", "Start a web server")
	webListenAddr := "127.0.0.1:3000"
	webC.Arg("listen-addr", "Web Server listen address").Default(webListenAddr).StringVar(&webListenAddr)

	if debug {
		utils.InitLogger(utils.LoggingForCLI, logrus.DebugLevel)
	}

	cmd, err := app.Parse(os.Args[1:])
	if err != nil {
		utils.FatalError(err)
	}

	switch cmd {
	case webC.FullCommand():
		err := runWebServer(ctx, webListenAddr, "./fixtures/cert.pem", "./fixtures/key.pem")
		if err != nil {
			utils.FatalError(err)
		}
	default:
		utils.FatalError(trace.BadParameter("unsupported command: %v", cmd))
	}
}

// setupSignalHandlers sets up a handler to handle common unix process signal traps.
// Some signals are handled to avoid the default handling which might be termination (SIGPIPE, SIGHUP, etc)
// The rest are considered as termination signals and the handler initiates shutdown upon receiving
// such a signal.
func setupSignalHandlers() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	go func() {
		defer cancel()
		for sig := range c {
			fmt.Printf("Received a %s signal, exiting...\n", sig)
			return
		}
	}()
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	return ctx
}
