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

package main

import (
	"context"

	"github.com/gravitational/teleport/integration/autoupdate/tools/updater"
	"github.com/gravitational/teleport/lib/modules"
	stacksignal "github.com/gravitational/teleport/lib/utils/signal"
	tctl "github.com/gravitational/teleport/tool/tctl/common"
)

func main() {
	ctx, cancel := stacksignal.GetSignalHandler().NotifyContext(context.Background())
	defer cancel()

	modules.SetInsecureTestMode(true)
	modules.SetModules(&updater.TestModules{})

	tctl.Run(ctx, tctl.Commands())
}
