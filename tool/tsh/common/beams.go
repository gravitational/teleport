package common

import (
	"github.com/alecthomas/kingpin/v2"
)

type beamsCommands struct {
}

func newBeamsCommands(
	app *kingpin.Application,
) beamsCommands {
	cmd := app.Command("beams", "View, manage and run beam environments. Beams are convenient, sandboxed environments for experimenting with AI agents.")
	cmd.Alias("beam")
	cmds := beamsCommands{
	}
	return cmds
}
