/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/automaticupgrades/installer"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

type AgentInstallCommand struct {
	config     *servicecfg.Config
	installCmd *kingpin.CmdClause

	version    string
	proxy      string
	enterprise bool
}

func (c *AgentInstallCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	c.config = config
	c.installCmd = app.Command("agent-install", "Install the teleport agent.").Hidden()
	c.installCmd.Flag("version", "Select the target install version.").StringVar(&c.version)
	c.installCmd.Flag("proxy", "Teleport proxy address.").StringVar(&c.proxy)
	c.installCmd.Flag("enterprise", "Installs enterprise version of Teleport.").Default("true").BoolVar(&c.enterprise)
}

func (c *AgentInstallCommand) TryRun(ctx context.Context, _ string, _ *auth.Client) (match bool, err error) {
	return true, trace.Wrap(c.AgentInstall(ctx))
}

func (c *AgentInstallCommand) selectVersion(_ context.Context) (string, error) {
	if c.version != "" {
		return c.version, nil
	}
	return "v15.1.10", nil
}

func (c *AgentInstallCommand) AgentInstall(ctx context.Context) error {
	version, err := c.selectVersion(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	flavor := "teleport"
	if c.enterprise {
		flavor = "teleport-ent"
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return trace.Wrap(err)
	}

	binDir := filepath.Join(homeDir, ".teleport", "bin", fmt.Sprintf("%s-%s", flavor, version))
	if err := os.MkdirAll(binDir, os.ModePerm); err != nil {
		return trace.Wrap(err)
	}

	// Do not attempt installation if valid teleport installation already exists
	if err := verifyInstallation(ctx, filepath.Join(binDir, "teleport"), version); err == nil {
		return nil
	}

	teleportInstaller, err := installer.NewTeleportInstaller(installer.Config{
		TeleportBinDir: binDir,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = teleportInstaller.InstallTeleport(ctx, installer.Request{
		Version: version,
		Arch:    runtime.GOARCH,
		OS:      runtime.GOOS,
		Flavor:  "teleport-ent",
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = verifyInstallation(ctx, filepath.Join(binDir, "teleport"), version)
	return trace.Wrap(err)
}

func verifyInstallation(ctx context.Context, binPath, expected string) error {
	if _, err := os.Stat(binPath); err != nil {
		return trace.Wrap(err)
	}
	version, err := exec.CommandContext(ctx, binPath, "version", "--raw").Output()
	if err != nil {
		return trace.Wrap(err)
	}
	expected = strings.TrimPrefix(expected, "v")
	actual := strings.TrimSpace(string(version))
	if expected != actual {
		return trace.Errorf("installed version does not match. expected: %s, actual %s", expected, actual)
	}
	return nil
}
