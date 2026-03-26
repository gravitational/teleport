/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"fmt"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
)

type beamsCommands struct {
	ls        *beamsLSCommand
	add       *beamsAddCommand
	rm        *beamsRMCommand
	console   *beamsConsoleCommand
	exec      *beamsExecCommand
	publish   *beamsPublishCommand
	unpublish *beamsUnpublishCommand
	cp        *beamsCPCommand
	allow     *beamsAllowCommand
	deny      *beamsDenyCommand
}

func newBeamsCommands(app *kingpin.Application) beamsCommands {
	beams := app.Command("beams", "View, manage and run beam instances. Beams are convenient, sandboxed environments for experimenting with AI agents.").Alias("beam")
	return beamsCommands{
		ls:        newBeamsLSCommand(beams),
		add:       newBeamsAddCommand(beams),
		rm:        newBeamsRMCommand(beams),
		console:   newBeamsConsoleCommand(beams),
		exec:      newBeamsExecCommand(beams),
		publish:   newBeamsPublishCommand(beams),
		unpublish: newBeamsUnpublishCommand(beams),
		cp:        newBeamsCPCommand(beams),
		allow:     newBeamsAllowCommand(beams),
		deny:      newBeamsDenyCommand(beams),
	}
}

type beamsLSCommand struct {
	*kingpin.CmdClause
	all    bool
	format string
}

func newBeamsLSCommand(parent *kingpin.CmdClause) *beamsLSCommand {
	cmd := &beamsLSCommand{
		CmdClause: parent.Command("ls", "List beam instances.").Alias("list"),
	}
	cmd.Flag("all", "List beams for all users.").BoolVar(&cmd.all)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	return cmd
}

func (c *beamsLSCommand) run(*CLIConf) error {
	fmt.Printf("tsh beams ls: all=%t format=%q\n", c.all, c.format)
	return trace.NotImplemented("tsh beams ls is not implemented yet")
}

type beamsAddCommand struct {
	*kingpin.CmdClause
	console bool
	format  string
}

func newBeamsAddCommand(parent *kingpin.CmdClause) *beamsAddCommand {
	cmd := &beamsAddCommand{
		CmdClause: parent.Command("add", "Start a new beam instance."),
	}
	cmd.Flag("console", "Connect to the beam after creation.").Default("true").BoolVar(&cmd.console)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	return cmd
}

func (c *beamsAddCommand) run(*CLIConf) error {
	fmt.Printf("tsh beams add: console=%t format=%q\n", c.console, c.format)
	return trace.NotImplemented("tsh beams add is not implemented yet")
}

type beamsRMCommand struct {
	*kingpin.CmdClause
	name   string
	format string
}

func newBeamsRMCommand(parent *kingpin.CmdClause) *beamsRMCommand {
	cmd := &beamsRMCommand{
		CmdClause: parent.Command("rm", "Delete a beam instance."),
	}
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	cmd.Arg("name", "Name (or alias) of the beam to delete.").Required().StringVar(&cmd.name)
	return cmd
}

func (c *beamsRMCommand) run(*CLIConf) error {
	fmt.Printf("tsh beams rm: name=%q format=%q\n", c.name, c.format)
	return trace.NotImplemented("tsh beams rm is not implemented yet")
}

type beamsConsoleCommand struct {
	*kingpin.CmdClause
	name string
}

func newBeamsConsoleCommand(parent *kingpin.CmdClause) *beamsConsoleCommand {
	cmd := &beamsConsoleCommand{
		CmdClause: parent.Command("console", "Start an interactive shell in a beam instance."),
	}
	cmd.Arg("name", "Name (or alias) of the beam to connect to.").Required().StringVar(&cmd.name)
	return cmd
}

func (c *beamsConsoleCommand) run(*CLIConf) error {
	fmt.Printf("tsh beams console: name=%q\n", c.name)
	return trace.NotImplemented("tsh beams console is not implemented yet")
}

type beamsExecCommand struct {
	*kingpin.CmdClause
	name    string
	command []string
}

func newBeamsExecCommand(parent *kingpin.CmdClause) *beamsExecCommand {
	cmd := &beamsExecCommand{
		CmdClause: parent.Command("exec", "Run a command in a beam instance."),
	}
	cmd.Arg("name", "Name (or alias) of the beam to target.").Required().StringVar(&cmd.name)
	cmd.Arg("command", "Command to execute in the instance.").Required().StringsVar(&cmd.command)
	return cmd
}

func (c *beamsExecCommand) run(*CLIConf) error {
	fmt.Printf("tsh beams exec: name=%q command=%q\n", c.name, c.command)
	return trace.NotImplemented("tsh beams exec is not implemented yet")
}

type beamsPublishCommand struct {
	*kingpin.CmdClause
	name   string
	tcp    bool
	format string
}

func newBeamsPublishCommand(parent *kingpin.CmdClause) *beamsPublishCommand {
	cmd := &beamsPublishCommand{
		CmdClause: parent.Command("publish", "Serve an HTTP or TCP service in a beam instance."),
	}
	cmd.Flag("tcp", "Publish as a TCP app instead of an HTTP app.").BoolVar(&cmd.tcp)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	cmd.Arg("name", "Name (or alias) of the beam to target.").Required().StringVar(&cmd.name)
	return cmd
}

func (c *beamsPublishCommand) run(*CLIConf) error {
	fmt.Printf("tsh beams publish: name=%q tcp=%t format=%q\n", c.name, c.tcp, c.format)
	return trace.NotImplemented("tsh beams publish is not implemented yet")
}

type beamsUnpublishCommand struct {
	*kingpin.CmdClause
	name   string
	format string
}

func newBeamsUnpublishCommand(parent *kingpin.CmdClause) *beamsUnpublishCommand {
	cmd := &beamsUnpublishCommand{
		CmdClause: parent.Command("unpublish", "Remove a previously served service in a beam instance."),
	}
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	cmd.Arg("name", "Name (or alias) of the beam to target.").Required().StringVar(&cmd.name)
	return cmd
}

func (c *beamsUnpublishCommand) run(*CLIConf) error {
	fmt.Printf("tsh beams unpublish: name=%q format=%q\n", c.name, c.format)
	return trace.NotImplemented("tsh beams unpublish is not implemented yet")
}

type beamsCPCommand struct {
	*kingpin.CmdClause
	recursive bool
	quiet     bool
	src       string
	dest      string
	format    string
}

func newBeamsCPCommand(parent *kingpin.CmdClause) *beamsCPCommand {
	cmd := &beamsCPCommand{
		CmdClause: parent.Command("cp", "Copy files between a beam instance and the local filesystem."),
	}
	cmd.Flag("recursive", "Recursive copy of subdirectories.").Short('r').BoolVar(&cmd.recursive)
	cmd.Flag("quiet", "Quiet mode.").Short('q').BoolVar(&cmd.quiet)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	cmd.Arg("src", "Source path to copy.").Required().StringVar(&cmd.src)
	cmd.Arg("dest", "Destination path to copy.").Required().StringVar(&cmd.dest)
	return cmd
}

func (c *beamsCPCommand) run(*CLIConf) error {
	if err := c.validate(); err != nil {
		return err
	}
	fmt.Printf("tsh beams cp: src=%q dest=%q recursive=%t quiet=%t format=%q\n", c.src, c.dest, c.recursive, c.quiet, c.format)
	return trace.NotImplemented("tsh beams cp is not implemented yet")
}

func (c *beamsCPCommand) validate() error {
	paths := []string{c.src, c.dest}

	var beamPaths int
	for _, path := range paths {
		if isBeamCopyPath(path) {
			beamPaths++
		}
	}

	if beamPaths != 1 {
		return trace.BadParameter("exactly one path must use the form BEAM_ID:PATH")
	}

	return nil
}

func isBeamCopyPath(path string) bool {
	beamID, beamPath, ok := strings.Cut(path, ":")
	return ok && beamID != "" && beamPath != ""
}

type beamsAllowCommand struct {
	*kingpin.CmdClause
	name   string
	domain string
	format string
}

func newBeamsAllowCommand(parent *kingpin.CmdClause) *beamsAllowCommand {
	cmd := &beamsAllowCommand{
		CmdClause: parent.Command("allow", "Grant access to an external domain."),
	}
	cmd.Flag("domain", "FQDN of a domain to allow access to.").Required().StringVar(&cmd.domain)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	cmd.Arg("name", "Name (or alias) of the beam to target.").Required().StringVar(&cmd.name)
	return cmd
}

func (c *beamsAllowCommand) run(*CLIConf) error {
	fmt.Printf("tsh beams allow: name=%q domain=%q format=%q\n", c.name, c.domain, c.format)
	return trace.NotImplemented("tsh beams allow is not implemented yet")
}

type beamsDenyCommand struct {
	*kingpin.CmdClause
	name   string
	domain string
	format string
}

func newBeamsDenyCommand(parent *kingpin.CmdClause) *beamsDenyCommand {
	cmd := &beamsDenyCommand{
		CmdClause: parent.Command("deny", "Remove access to an external domain."),
	}
	cmd.Flag("domain", "FQDN of a domain to deny access to.").Required().StringVar(&cmd.domain)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	cmd.Arg("name", "Name (or alias) of the beam to target.").Required().StringVar(&cmd.name)
	return cmd
}

func (c *beamsDenyCommand) run(*CLIConf) error {
	fmt.Printf("tsh beams deny: name=%q domain=%q format=%q\n", c.name, c.domain, c.format)
	return trace.NotImplemented("tsh beams deny is not implemented yet")
}
