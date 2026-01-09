package main

import (
	"os"
	"runtime"

	"github.com/gravitational/teleport/lib/srv/reexec"
)

func init() {
	// this binary shouldn't do much
	runtime.GOMAXPROCS(1)
	// PAM and other process shenanigans might be relying on being on a specific
	// OS thread; in theory they should be calling LockOSThread already, but let's
	// be defensive and do it ourselves here during init too
	runtime.LockOSThread()
}

func main() {

	// this binary ends up being called as /proc/<n>/fd/<m> which results in the
	// file descriptor number of the original process being used as the "comm"
	// value, which is displayed in pstree and other similar tools, so we set it
	// to a sensible value here instead
	// TODO(espadolini): do this with /proc/<pid>/comm in the parent?
	_ = os.WriteFile("/proc/self/comm", []byte("teleport-sshd-helper\x00"), 0)

	reexec.RunAndExit(os.Args[1])
}
