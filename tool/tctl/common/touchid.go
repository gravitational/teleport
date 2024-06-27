// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package common

import (
	"context"

	"github.com/alecthomas/kingpin/v2"

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/tool/common/touchid"
)

// touchIDCommand adapts touchid.Command for tclt.
type touchIDCommand struct {
	impl *touchid.Command
}

func (c *touchIDCommand) Initialize(app *kingpin.Application, _ *servicecfg.Config) {
	c.impl = touchid.NewCommand(app)
}

func (c *touchIDCommand) TryRun(ctx context.Context, selectedCommand string, _ *authclient.Client) (match bool, err error) {
	return c.impl.TryRun(ctx, selectedCommand)
}
