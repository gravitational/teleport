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

const (
	GlobalHelpString = "Admin tool for the Teleport Infrastructure Identity Platform"

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
  The generated token will be valid for 30 minutes.

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
