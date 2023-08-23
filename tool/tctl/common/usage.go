/*
Copyright 2015-2021 Gravitational, Inc.

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

const (
	GlobalHelpString = "Admin tool for the Teleport Access Platform"

	AddUserHelp = `Notes:

  1. tctl will generate a signup token and give you a URL to share with a user.
     A user will have to complete account creation by visiting the URL.

  2. The allowed logins of the account only apply if a role uses them by including
     '{{ internal.logins }}' variable in a role definition. The same is true for
     the allowed Windows logins and the '{{ internal.windows_logins }}' variable.

Examples:

  > tctl users add --roles=editor,dba joe

  This creates a Teleport account 'joe' who will assume the roles 'editor' and 'dba'
  To see the permissions of 'editor' role, execute 'tctl get role/editor'
`

	AddNodeHelp = `Notes:
  This command generates and prints an invitation token another node can use to
  join the cluster.

Examples:

  > tctl nodes add

  Generates a token when can be used to add a regular SSH node to the cluster.
  The token genrated single-use token will be valid for 30 minutes.

  > tctl nodes add --roles=node,proxy --ttl=1h

  Generates a token when can be used to add an SSH node to the cluster which
  will also be a proxy node. This token can be used multiple times within an
  hour.
`
	ListNodesHelp = `Notes:
  SSH nodes send periodic heartbeat to the Auth service. This command prints
  the list of current online nodes.
`
)
