// Copyright 2022 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"github.com/gravitational/teleport/tool/tctl/common/loginrule"
	"github.com/gravitational/teleport/tool/tctl/sso/configure"
	"github.com/gravitational/teleport/tool/tctl/sso/tester"
)

// Commands returns the set of commands that are to oss and ent
// variants of tctl.
func Commands() []CLICommand {
	return []CLICommand{
		&UserCommand{},
		&NodeCommand{},
		&TokensCommand{},
		&AuthCommand{},
		&StatusCommand{},
		&TopCommand{},
		&AccessRequestCommand{},
		&AppsCommand{},
		&DBCommand{},
		&KubeCommand{},
		&DesktopCommand{},
		&LockCommand{},
		&BotsCommand{},
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
		&AutoUpdateCommand{},
	}
}

// OSSCommands returns the oss variants of commands that use different variants
// for oss and ent.
func OSSCommands() []CLICommand {
	return []CLICommand{
		&configure.SSOConfigureCommand{},
		&tester.SSOTestCommand{},
	}
}
