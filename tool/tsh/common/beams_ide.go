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
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/alecthomas/kingpin/v2"
	"github.com/charmbracelet/huh"
	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	ideVSCode = "vscode"
	ideCursor = "cursor"
	ideZed    = "zed"

	// beamsIDEHomeDir is the home directory of the beams login user.
	beamsIDEHomeDir = "/home/" + types.BeamsLogin
)

// beamsIDE describes an editor that `tsh beams ide` can open.
type beamsIDE struct {
	// id identifies the editor in the --ide flag.
	id string
	// name is the human-friendly name shown in the picker.
	name string
	// bin is the editor's CLI launcher, searched for on PATH.
	bin string
	// launchArgs returns the launcher arguments that open a remote SSH
	// session to target.
	launchArgs func(target, dir string) []string
}

// vscodeRemoteSSHArgs builds the arguments understood by VS Code and its forks.
func vscodeRemoteSSHArgs(target, dir string) []string {
	return []string{"--new-window", "--remote", "ssh-remote+" + target, dir}
}

// beamsIDEIDs are the accepted values of the --ide flag. Keep in sync with beamsIDEs.
var beamsIDEIDs = []string{ideVSCode, ideCursor, ideZed}

// beamsIDEs lists the supported editors in the order shown to the user.
var beamsIDEs = []beamsIDE{
	{id: ideVSCode, name: "Visual Studio Code", bin: "code", launchArgs: vscodeRemoteSSHArgs},
	{id: ideCursor, name: "Cursor", bin: "cursor", launchArgs: vscodeRemoteSSHArgs},
	{id: ideZed, name: "Zed", bin: "zed", launchArgs: func(target, dir string) []string {
		return []string{fmt.Sprintf("ssh://%s%s", target, dir)}
	}},
}

// beamsIDEByID returns the editor with the given id.
func beamsIDEByID(id string) (beamsIDE, bool) {
	for _, ide := range beamsIDEs {
		if ide.id == id {
			return ide, true
		}
	}
	return beamsIDE{}, false
}

// beamsIDECommand implements `tsh beams ide <beam> --ide=<editor>`. It opens the user's IDE
// of choice with a remote SSH session into the beam.
type beamsIDECommand struct {
	*kingpin.CmdClause

	// beam is either the alias or UUID of the beam to open. Both are supported
	beam string
	// ide is the id of the editor to open, from the --ide flag or the
	// TELEPORT_BEAMS_IDE environment variable.
	ide string

	// These helper functions can be overridden in tests.
	getBeamFn     func(context.Context, authclient.ClientI, string) (*beamsv1.Beam, error)
	withClusterFn func(context.Context, *client.TeleportClient, func(authclient.ClientI, string) error) error
	pickEditorFn  func(*CLIConf) (string, error)
}

func newBeamsIDECommand(parent *kingpin.CmdClause) *beamsIDECommand {
	cmd := &beamsIDECommand{
		CmdClause: parent.Command("ide", "Open your IDE with a remote SSH session into a beam."),
	}
	cmd.getBeamFn = getBeam
	cmd.withClusterFn = withRootCluster
	cmd.pickEditorFn = pickBeamsIDEEditor
	cmd.Arg("beam", "Beam to open, specified by ID or UUID.").Required().StringVar(&cmd.beam)
	cmd.Flag("ide", "IDE to open. Prompts for a choice when omitted.").
		Envar(beamsIDEEnvVar).
		EnumVar(&cmd.ide, beamsIDEIDs...)
	return cmd
}

func (c *beamsIDECommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyHost := tc.WebProxyHost()

	var beam *beamsv1.Beam
	var rootClusterName string
	err = c.withClusterFn(ctx, tc, func(root authclient.ClientI, clusterName string) error {
		rootClusterName = clusterName
		beam, err = c.getBeamFn(ctx, root, c.beam)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	nodeID, err := beamNodeID(beam)
	if err != nil {
		return trace.Wrap(err)
	}
	alias := beam.GetStatus().GetAlias()

	editorID := c.ide
	if editorID == "" {
		editorID, err = c.pickEditorFn(cf)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintf(cf.Stderr(), "Tip: pass --ide=%[1]s or set %[2]s=%[1]s to skip this prompt.\n", editorID, beamsIDEEnvVar)
	}

	ide, ok := beamsIDEByID(editorID)
	if !ok {
		return trace.BadParameter("unsupported IDE %q", editorID)
	}

	binPath, err := cf.LookPath(ide.bin)
	if err != nil {
		return trace.NotFound(
			"%q was not found in PATH. Install %v and make sure its CLI launcher (%q) is available, or pass --ide=<editor> to open a different IDE",
			ide.bin, ide.name, ide.bin)
	}

	sshTarget := fmt.Sprintf("%s@%s.%s", types.BeamsLogin, nodeID, rootClusterName)
	args := ide.launchArgs(sshTarget, beamsIDEHomeDir)

	cmd := exec.Command(binPath, args...)
	cmd.Stdout = cf.Stdout()
	cmd.Stderr = cf.Stderr()
	if err := cf.RunCommand(cmd); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(cf.Stderr(), "Opening %v with a remote SSH session into beam %q.\n", ide.name, alias)
	fmt.Fprintf(cf.Stderr(),
		"If the IDE fails to connect, check its Remote - SSH logs and make sure your OpenSSH config includes the Teleport configuration for %v (run `tsh config --proxy %v` and add its output to your OpenSSH config).\n",
		rootClusterName, proxyHost)
	return nil
}

// pickBeamsIDEEditor prompts the user to select an IDE from an interactive list. Requires stdin to be a terminal.
func pickBeamsIDEEditor(cf *CLIConf) (string, error) {
	if !utils.IsTerminalReader(cf.Stdin()) {
		return "", trace.BadParameter("no IDE selected: pass --ide")
	}

	options := make([]huh.Option[string], 0, len(beamsIDEs))
	for _, ide := range beamsIDEs {
		options = append(options, huh.NewOption(ide.name, ide.id))
	}

	var editor string
	sel := huh.NewSelect[string]().
		Title("Choose an IDE to open").
		Options(options...).
		Value(&editor)
	sel.WithTheme(huh.ThemeBase16())
	if err := sel.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", trace.BadParameter("no IDE selected")
		}
		return "", trace.Wrap(err)
	}
	return editor, nil
}
