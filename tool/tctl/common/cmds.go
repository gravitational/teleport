/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"github.com/gravitational/teleport/tool/tctl/common/accessmonitoring"
	"github.com/gravitational/teleport/tool/tctl/common/decision"
	"github.com/gravitational/teleport/tool/tctl/common/loginrule"
	"github.com/gravitational/teleport/tool/tctl/common/plugin"
	"github.com/gravitational/teleport/tool/tctl/common/top"
	"github.com/gravitational/teleport/tool/tctl/sso/configure"
	"github.com/gravitational/teleport/tool/tctl/sso/tester"
)

// Commands returns the set of available subcommands for tctl.
func Commands() []CLICommand {
	return []CLICommand{
		&VersionCommand{},
		&UserCommand{},
		&NodeCommand{},
		&TokensCommand{},
		&AuthCommand{},
		&StatusCommand{},
		&top.Command{},
		&AccessRequestCommand{},
		&AppsCommand{},
		&DBCommand{},
		&KubeCommand{},
		&DesktopCommand{},
		&LockCommand{},
		&BotsCommand{},
		&WorkloadIdentityCommand{},
		&InventoryCommand{},
		&RecordingsCommand{},
		&AlertCommand{},
		&ProxyCommand{},
		&ResourceCommand{},
		&EditCommand{},
		&ExternalAuditStorageCommand{},
		&LoadtestCommand{},
		&DevicesCommand{},
		&SAMLCommand{},
		&ACLCommand{},
		&loginrule.Command{},
		&IdPCommand{},
		&accessmonitoring.Command{},
		&plugin.PluginsCommand{},
		&NotificationCommand{},
		&configure.SSOConfigureCommand{},
		&tester.SSOTestCommand{},
		&fido2Command{},
		&webauthnwinCommand{},
		&touchIDCommand{},
		&TerraformCommand{},
		&AutoUpdateCommand{},
		&decision.Command{},
	}
}
