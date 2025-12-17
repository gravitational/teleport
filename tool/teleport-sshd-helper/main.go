package main

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/gravitational/teleport/lib/srv/reexec"
)

func init() {
	runtime.GOMAXPROCS(1)
}

func main() {
	l, err := net.ListenUnix("unix", &net.UnixAddr{
		Net:  "unix",
		Name: fmt.Sprintf("\x00teleport-sshd-helper/%d", os.Getpid()),
	})
	if err != nil {
		panic(err)
	}
	go func() {
		http.Serve(l, nil)
	}()

	reexec.RunAndExit(os.Args[1])
}
