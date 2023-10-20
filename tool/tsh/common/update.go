/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/automaticupgrades/client"
)

type updateCommand struct {
	*kingpin.CmdClause

	updateConfirmed bool
	releaseChannel  string
}

// newUpdateCommand returns the update subcommand.
func newUpdateCommand(app *kingpin.Application) *updateCommand {
	updateCmd := &updateCommand{
		CmdClause: app.Command("update", "Update Teleport client tools."),
	}
	updateCmd.Flag("yes", "Update without interactive prompt (default false)").BoolVar(&updateCmd.updateConfirmed)
	updateCmd.Flag("channel", "Specify a custom release channel").StringVar(&updateCmd.releaseChannel)
	return updateCmd
}

func (c *updateCommand) run(cf *CLIConf) error {
	if err := verifyPermissions(); err != nil {
		return trace.Wrap(err)
	}

	version, err := desiredVersion(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	if teleport.Version == version {
		fmt.Printf("tsh is running %s; no update required\n", teleport.Version)
		return nil
	}

	if !c.confirmUpdate(version) {
		fmt.Println("Update canceled")
		return nil
	}

	updater, err := client.NewUpdater(client.UpdaterConfig{
		TeleportVersion: version,
		ReleaseChannel:  c.releaseChannel,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := updater.Update(cf.Context); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// confirmUpdate returns true if user has confirmed the update
func (c *updateCommand) confirmUpdate(version string) bool {
	if c.updateConfirmed {
		return true
	}

	var input string
	fmt.Printf("Updating tsh from %s to %s\n", teleport.Version, version)
	fmt.Print("Do you want to continue? [Y/n] ")
	fmt.Scanln(&input)

	switch {
	case input == "", strings.HasPrefix(strings.ToLower(input), "y"):
		return true
	default:
		return false
	}
}

// desiredVersion returns the desired Teleport version
func desiredVersion(ctx context.Context) (string, error) {
	stableVersion, err := automaticupgrades.Version(ctx, "")
	if err != nil {
		return "", trace.Wrap(err, "failed to fetch automatic upgrade version")
	}
	return strings.TrimPrefix(stableVersion, "v"), nil
}

// verifyPermissions verifies the user has the required permissions to update tsh
func verifyPermissions() error {
	if os.Geteuid() != 0 {
		return trace.AccessDenied("update requires root/sudo privileges")
	}
	return nil
}
