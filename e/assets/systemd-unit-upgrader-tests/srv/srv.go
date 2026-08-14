package main

import (
	"flag"
	"fmt"
	"os"

	unit "github.com/gravitational/teleport/e/assets/systemd-unit-upgrader-tests"
)

func main() {
	version := flag.String("version", "1.2.3", "set the version value")
	critical := flag.String("critical", "no", "set the critical value")
	path := flag.String("path", "/v1/stable/cloud", "set the base path")
	addr := flag.String("addr", ":8000", "set server addr")

	flag.Parse()

	normPath := *path
	if normPath[0:1] != "/" {
		normPath = "/" + normPath
	}

	end := unit.NewUpgradeEndpoint(normPath)
	end.Addr = *addr
	end.SetPath(normPath)
	end.SetVersion(*version)
	end.SetCritical(*critical)

	fmt.Fprintf(os.Stderr, "Serving upgrade endpoint %s%s...\n", *addr, normPath)
	end.ListenAndServe()
}
