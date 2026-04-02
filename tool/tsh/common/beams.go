package common

import (
	"github.com/alecthomas/kingpin/v2"
)

type beamsCommands struct {
	ls      *beamsListCommand
	add     *beamsAddCommand
	rm      *beamsRemoveCommand
	shell   *beamsShellCommand
	mount   *beamsMountCommand
	unmount *beamsUnmountCommand
	allow   *beamsAllowCommand
	deny    *beamsDenyCommand
}

func newBeamsCommands(
	app *kingpin.Application,
) beamsCommands {
	cmd := app.Command("beams", "View, manage and run beam environments. Beams are convenient, sandboxed environments for experimenting with AI agents.")
	cmd.Alias("beam")
	cmds := beamsCommands{
		ls:      newBeamsListCommand(cmd),
		add:     newBeamsAddCommand(cmd),
		rm:      newBeamsRemoveCommand(cmd),
		shell:   newBeamsShellCommand(cmd),
		mount:   newBeamsMountCommand(cmd),
		unmount: newBeamsUnmountCommand(cmd),
		allow:   newBeamsAllowCommand(cmd),
		deny:    newBeamsDenyCommand(cmd),
	}
	return cmds
}
