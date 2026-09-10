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
	"fmt"
	"os"
	"path/filepath"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/utils"
)

type mcpLogoutCommand struct {
	*kingpin.CmdClause
	cf *CLIConf
}

func newMCPLogoutCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpLogoutCommand {
	cmd := &mcpLogoutCommand{
		CmdClause: parent.Command("logout", "Remove stored OAuth credentials for an MCP server."),
		cf:        cf,
	}
	cmd.Arg("name", "Name of the MCP server. Removes credentials for all MCP servers when omitted.").SetValue(&cf.AppSQN)
	return cmd
}

// Avoid an app lookup so credentials remain removable after an app is deleted.
func (c *mcpLogoutCommand) run() error {
	tc, err := makeClient(c.cf)
	if err != nil {
		return trace.Wrap(err)
	}
	lockPath := mcpOAuthMutationLockPath(c.cf.HomePath)

	if c.cf.AppSQN.Name == "" {
		dir := keypaths.AppCredentialDir(profile.FullProfilePath(c.cf.HomePath), tc.WebProxyHost(), tc.Username, tc.SiteName)
		removed, err := removeAllMCPOAuthCredentials(c.cf.Context, dir, lockPath)
		if err != nil {
			return trace.Wrap(err)
		}
		if removed == 0 {
			fmt.Fprintln(c.cf.Stdout(), "No OAuth credentials stored for any MCP server.")
			return nil
		}
		fmt.Fprintln(c.cf.Stdout(), "Removed OAuth credentials for all MCP servers.")
		return nil
	}

	appName := c.cf.AppSQN.String()
	credsPath, err := mcpOAuthTokenPath(c.cf.HomePath, tc.WebProxyHost(), tc.Username, tc.SiteName, c.cf.AppSQN)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := removeMCPOAuthCredentials(c.cf.Context, credsPath, lockPath); err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("no OAuth credentials stored for MCP server %q", appName)
		}
		return trace.Wrap(err)
	}
	fmt.Fprintf(c.cf.Stdout(), "Removed OAuth credentials for MCP server %q.\n", appName)
	return nil
}

// Keep the lock inode stable so concurrent refreshers cannot recreate credentials.
func removeMCPOAuthCredentials(ctx context.Context, path, mutationLockPath string) error {
	unlock, err := utils.FSTryWriteLockTimeout(ctx, path+".lock", mcpOAuthAuthorizationTimeout+mcpOAuthRefreshLockTimeout)
	if err != nil {
		return trace.Wrap(err)
	}
	defer unlock()
	return withMCPOAuthMutationLock(ctx, mutationLockPath, func() error {
		return trace.ConvertSystemError(os.Remove(path))
	})
}

// removeAllMCPOAuthCredentials removes every credentials file in dir and
// returns how many it removed. A missing dir means there is nothing to remove.
func removeAllMCPOAuthCredentials(ctx context.Context, dir, mutationLockPath string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return 0, trace.ConvertSystemError(err)
	}
	removed := 0
	var errs []error
	for _, entry := range entries {
		if !keypaths.IsMCPOAuthCredentialsFile(entry.Name()) {
			continue
		}
		if err := removeMCPOAuthCredentials(ctx, filepath.Join(dir, entry.Name()), mutationLockPath); err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}
		removed++
	}
	return removed, trace.NewAggregate(errs...)
}
