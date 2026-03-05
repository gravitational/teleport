package init

import (
	"os"

	"github.com/gravitational/teleport/lib/srv/reexec"
)

var _ = func() struct{} {
	println("this is an IIFE for a var _ declaration")
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case reexec.ExecSubCommand, reexec.NetworkingSubCommand, reexec.CheckHomeDirSubCommand, reexec.ParkSubCommand, reexec.SFTPSubCommand:
			reexec.RunAndExit(os.Args[1])
		}
	}
	return struct{}{}
}()
