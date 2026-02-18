package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsVibeCommand struct {
	*kingpin.CmdClause
}

const helpLong = `Start an ephemeral vibe-coding session in a sandboxed environment.

A convenience for;
    tsh beams add my-beam --wait
    tsh beams mount my-beam --source=./ --dest=/mnt/workspace
    tsh beams expose my-beam --mode=http --port=8080
    tsh beams shell my-beam
    tsh beams unmount my-beam --dest=/mnt/workspace
    tsh beams rm my-beam
`

func newBeamsVibeCommand(parent *kingpin.CmdClause) *beamsVibeCommand {
	cmd := &beamsVibeCommand{
		CmdClause: parent.Command("vibe", "Start an ephemeral vibe-coding session in a sandboxed environment."),
	}
	return cmd
}

func (c *beamsVibeCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	beam, err := tc.AddBeam(ctx, "", true)
	if err != nil {
		return trace.Wrap(err, "adding beam")
	}

	err = tc.MountBeam(ctx, beam.Name, "./", "/mnt/workspace")
	if err != nil {
		return trace.Wrap(err, "mounting beam workspace")
	}

	err = tc.ExposeBeam(ctx, beam.Name, "http", 8080, "")
	if err != nil {
		return trace.Wrap(err, "exposing beam service")
	}

	err = tc.ShellBeam(ctx, beam.Name) // Returns when the shell session ends
	if err != nil {
		return trace.Wrap(err, "shelling to beam")
	}

	err = tc.UnmountBeam(ctx, beam.Name, "/mnt/workspace")
	if err != nil {
		return trace.Wrap(err, "unmounting beam workspace")
	}

	err = tc.RemoveBeam(ctx, beam.Name)
	if err != nil {
		return trace.Wrap(err, "removing beam")
	}

	return trace.Wrap(err)
}
