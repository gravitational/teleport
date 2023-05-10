package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/prompt"
	"github.com/gravitational/teleport/tool/common"
	tshcommon "github.com/gravitational/teleport/tool/tsh/common"
)

func main() {
	cmdLineOrig := os.Args[1:]
	var cmdLine []string

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// lets see: if the executable name is 'ssh' or 'scp' we convert
	// that to "tsh ssh" or "tsh scp"
	switch path.Base(os.Args[0]) {
	case "ssh":
		cmdLine = append([]string{"ssh"}, cmdLineOrig...)
	case "scp":
		cmdLine = append([]string{"scp"}, cmdLineOrig...)
	default:
		cmdLine = cmdLineOrig
	}

	err := tshcommon.Run(ctx, cmdLine)
	prompt.NotifyExit() // Allow prompt to restore terminal state on exit.
	if err != nil {
		var exitError *common.ExitCodeError
		if errors.As(err, &exitError) {
			os.Exit(exitError.Code)
		}
		utils.FatalError(err)
	}
}
