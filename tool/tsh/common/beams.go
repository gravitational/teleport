package common

import (
	"github.com/alecthomas/kingpin/v2"
)

type beamsCommands struct {
	ls      *beamsListCommand
	add     *beamsAddCommand
	rm      *beamsRemoveCommand
	exec    *beamsExecCommand
	shell   *beamsShellCommand
	mount   *beamsMountCommand
	unmount *beamsUnmountCommand
	expose  *beamsExposeCommand
	push    *beamsPushCommand
	pull    *beamsPullCommand
	allow   *beamsAllowCommand
	deny    *beamsDenyCommand
	vibe    *beamsVibeCommand
	run     *beamsRunCommand
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
		exec:    newBeamsExecCommand(cmd),
		shell:   newBeamsShellCommand(cmd),
		mount:   newBeamsMountCommand(cmd),
		unmount: newBeamsUnmountCommand(cmd),
		expose:  newBeamsExposeCommand(cmd),
		push:    newBeamsPushCommand(cmd),
		pull:    newBeamsPullCommand(cmd),
		allow:   newBeamsAllowCommand(cmd),
		deny:    newBeamsDenyCommand(cmd),
		vibe:    newBeamsVibeCommand(cmd),
		run:     newBeamsRunCommand(cmd),
	}
	return cmds
}
