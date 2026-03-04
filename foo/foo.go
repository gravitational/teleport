package main

import (
	"os"

	"github.com/gravitational/teleport/lib/srv"
)

func main() {
	srv.RunAndExit(os.Args[1])
}
