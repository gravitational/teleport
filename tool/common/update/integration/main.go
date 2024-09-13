package main

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"

	"github.com/gravitational/teleport/tool/common/update"
)

var version = "development"

func main() {
	// At process startup, check if a version has already been downloaded to
	// $TELEPORT_HOME/bin or if the user has set the TELEPORT_TOOLS_VERSION
	// environment variable. If so, re-exec that version of {tsh, tctl}.
	toolsVersion, reExec := update.CheckLocal()
	if reExec {
		// Download the version of client tools required by the cluster. This
		// is required if the user passed in the TELEPORT_TOOLS_VERSION
		// explicitly.
		err := update.Download(toolsVersion)
		if errors.Is(err, update.ErrCancelled) {
			os.Exit(0)
			return
		}
		if err != nil {
			log.Fatalf("Failed to download version (%v): %v", toolsVersion, err)
			return
		}

		// Re-execute client tools with the correct version of client tools.
		code, err := update.Exec()
		if err != nil {
			log.Fatalf("Failed to re-exec client tool: %v", err)
		} else {
			os.Exit(code)
		}
	}
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("Teleport v%v git", version)
	}
}
