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
	// loginUsageFooter is printed at the bottom of `tsh help login` output
	loginUsageFooter = `NOTES:
  The proxy address format is host:https_port,ssh_proxy_port

  Passwordless only works in local auth
  --auth=passwordless flag can be omitted if your cluster configuration set the connector_name: passwordless option.

EXAMPLES:
  Use ports 8080 and 8023 for https and SSH proxy:
  $ tsh --proxy=host.example.com:8080,8023 login

  Use port 8080 and 3023 (default) for SSH proxy:
  $ tsh --proxy=host.example.com:8080 login

  Login and select cluster "two":
  $ tsh --proxy=host.example.com login two

  Select cluster "two" using existing credentials and proxy:
  $ tsh login two

  For passwordless authentication use:
  $ tsh login --auth=passwordless`

	// emptyNodesFooter is printed at the bottom of `tsh ls` when no results are returned for nodes.
	emptyNodesFooter = `
  Not seeing nodes? Either no nodes are available or your user's roles do not match the labels of at least one node.

  Check with your Teleport cluster administrator that your user's roles should have nodes available.
  `

	dbListHelp = `
Examples:
  Search databases with keywords:
  $ tsh db ls --search foo,bar

  Filter databases with labels:
  $ tsh db ls key1=value1,key2=value2

  List databases from all clusters with extra fields:
  $ tsh db ls --all -v

  Get database names using "jq":
  $ tsh db ls --format json  | jq -r '.[].metadata.name'`

	dbExecHelp = `
Examples:
  Search databases with labels:
  $ tsh db exec "source my_script.sql" --db-user mysql --labels key1=value1,key2=value2

  Search databases with keywords:
  $ tsh db exec "select 1" --db-user mysql --db-name mysql --search foo,bar

  Execute a command on specified target databases without confirmation:
  $ tsh db exec "select @@hostname" --db-user mysql --dbs mydb1,mydb2,mydb3 --no-confirm

  Run commands in parallel, and save outputs to files:
  $ tsh db exec "select 1" --db-user mysql --labels env=dev --parallel=5 --output-dir=exec-outputs`
)
