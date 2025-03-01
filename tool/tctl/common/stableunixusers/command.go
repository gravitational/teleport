// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package stableunixusers

import (
	"context"
	"encoding/json"
	"os"
	"strconv"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	stableunixusersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/stableunixusers/v1"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/tool/tctl/common/client"
	"github.com/gravitational/teleport/tool/tctl/common/config"
)

// Command implements the commands under "tctl stable-unix-users".
type Command struct {
	lsCmd    *kingpin.CmdClause
	lsFormat string
}

// Initialize implements [tool/tctl/common.CLICommand].
func (c *Command) Initialize(app *kingpin.Application, _ *config.GlobalCLIFlags, _ *servicecfg.Config) {
	rootCmd := app.Command("stable-unix-users", "Manage the database of stable UNIX users.")
	c.lsCmd = rootCmd.Command("ls", "List the stable UNIX users currently persisted in the cluster.")
	c.lsCmd.Flag("format", "Output format, 'text', or 'json'").Default(teleport.Text).EnumVar(&c.lsFormat, teleport.Text, teleport.JSON, teleport.YAML)
}

// TryRun implements [tool/tctl/common.CLICommand].
func (c *Command) TryRun(ctx context.Context, fullCommand string, clientFunc client.InitFunc) (match bool, err error) {
	switch fullCommand {
	default:
		return false, nil
	case c.lsCmd.FullCommand():
		return true, trace.Wrap(c.ls(ctx, clientFunc))
	}
}

// ls is the implementation of "tctl stable-unix-users ls".
func (c *Command) ls(ctx context.Context, clientFunc client.InitFunc) error {
	authClient, close, err := clientFunc(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer close(ctx)

	clt := authClient.StableUNIXUsersClient()

	type userOut struct {
		Username string `json:"username"`
		UID      int32  `json:"uid"`
	}
	var usersOut []userOut

	var pageToken string
	for {
		resp, err := clt.ListStableUNIXUsers(ctx, &stableunixusersv1.ListStableUNIXUsersRequest{
			PageSize:  0,
			PageToken: pageToken,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		for _, user := range resp.GetStableUnixUsers() {
			usersOut = append(usersOut, userOut{
				Username: user.GetUsername(),
				UID:      user.GetUid(),
			})
		}
		pageToken = resp.GetNextPageToken()

		if pageToken == "" {
			break
		}
	}

	switch c.lsFormat {
	default:
		// format is an enum parameter, kingpin will check and error out if the
		// value is unknown long before getting to this point
		return trace.BadParameter("unknown format %q (this is a bug)", c.lsFormat)
	case teleport.Text:
		table := asciitable.MakeTable([]string{"Username", "UID"})
		for _, u := range usersOut {
			table.AddRow([]string{u.Username, strconv.Itoa(int(u.UID))})
		}
		if err := table.WriteTo(os.Stdout); err != nil {
			return trace.Wrap(err)
		}

	case teleport.JSON, teleport.YAML:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "    ")
		if usersOut == nil {
			// avoid encoding a nil slice as "null"
			usersOut = []userOut{}
		}
		if err := enc.Encode(usersOut); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
