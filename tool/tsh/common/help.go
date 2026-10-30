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

	mcpConfigHelp = `
Examples:
  Print sample configuration for a MCP server app
  $ tsh mcp config my-mcp-server-app

  Print sample configuration for a streamable HTTP MCP server with custom headers
  $ tsh mcp config my-mcp-server-app -H "Header1: value1" -H "Header2: value2"

  Add all MCP servers to Claude Desktop
  $ tsh mcp config --all --client-config=claude

  Add all MCP servers to Cursor
  $ tsh mcp config --all --client-config=cursor

  Generate config for all MCP servers for VSCode
  $ tsh mcp config --all --format=vscode

  Add all MCP servers to VSCode project
  $ tsh mcp config --all --client-config=<path-to-project>/.vscode/mcp.json

  Add all MCP servers to Claude Code project
  $ tsh mcp config --all --client-config=<path-to-project>/.mcp.json

  Search MCP servers with labels and add to the specified JSON file
  $ tsh mcp config --labels env=dev --client-config=my-config.json`

	mcpDBConfigHelp = `
Examples:
  Print sample configuration for exposing database as MCP server
  $ tsh mcp db config --db-user=mydbuser --db-name=mydbname my-db-resource

  Add the database configuration to Claude Desktop
  $ tsh mcp db config --db-user=mydbuser --db-name=mydbname --client-config=claude my-db-resource

  Add the database configuration to Cursor
  $ tsh mcp db config --db-user=mydbuser --db-name=mydbname --client-config=cursor my-db-resource

  Generate database configuration for VSCode
  $ tsh mcp db config --db-user=mydbuser --db-name=mydbname --format=vscode my-db-resource

  Add the database configuration to VSCode project
  $ tsh mcp db config --db-user=mydbuser --db-name=mydbname --client-config=<path-to-project>/.vscode/.mcp.json my-db-resource

  Add database configuration to Claude Code project
  $ tsh mcp db config --db-user=mydbuser --db-name=mydbname --client-config=<path-to-project>/.mcp.json my-db-resource

  Add the database configuration to the specified JSON file
  $ tsh mcp db config --db-user=mydbuser --db-name=mydbname --client-config=my-config.json my-db-resource
`

	beamsAddHelp = `
Examples:
  Create a beam and connect via SSH:
  $ tsh beams add

  Create a beam without connecting and save its ID:
  $ BEAM_ID=$(tsh beams add -f json | jq -r '.id')
  $ tsh beams exec $BEAM_ID -- ls examples`

	phaseAddHelp = `
Examples:
  Create a Phase and open an interactive build console:
  $ tsh phase add dev-tools

  Limit the roles available to this Phase's builds:
  $ tsh phase add dev-tools --roles=developer,database-access

  Start an interactive build without connecting, and return its build_beam ID:
  $ tsh phase add dev-tools --no-console --format=json

  Build a Phase from a raw command file:
  $ tsh phase add dev-tools --commands-file=build.yaml

  Start a command build and return after provisioning:
  $ tsh phase add dev-tools --commands-file=build.yaml --no-console

  Copy local files outside /home into a new Phase:
  $ tsh phase add app-files --copy=/opt/app/config:./config --copy=/etc/app.conf:./app.conf

  Use another Phase as the base image:
  $ tsh phase add project-tools --from=dev-tools

  From inside a build Beam, fork the current filesystem:
  $ tsh phase add my-fork --fork

  Fork another Phase's last saved interactive filesystem:
  $ tsh phase add my-fork --fork-from=dev-tools`

	phaseApplyHelp = `
Examples:
  Create or update every stage, in order, streaming command logs:
  $ tsh phase apply -f phase.yaml

  Finish every stage without streaming command logs:
  $ tsh phase apply -f phase.yaml --no-console

  Reapply after changing local files or commands. Existing stages keep their
  parent and cannot switch between files and commands. If a stage fails,
  completed stages remain saved.

Example phase.yaml:
version: v1
metadata:
  name: app-files                  # Unique within this file and the cluster.
                                   # Omit from here to use the default base image.
files:                             # Use files or commands, never both.
  - from: ./config                 # Local path, relative to this YAML file.
    to: /opt/app-config             # Required absolute destination outside /home.
                                   # Directories merge into the destination.
---                                # Separate stages with a YAML document separator.
version: v1
metadata:
  name: app
  from: app-files                   # Every later stage names the preceding stage.
commands:                          # Nonempty list, run in order as root outside /home.
  - comment: Install tools         # Displayed in build logs as >> Install tools.
    shell: apt-get update && apt-get install -y git curl
                                   # Or use command with an optional args list.
services:
  - name: prepare-config           # Replaces any inherited service with this name.
    type: pre-start
    context: user                  # Default: interactive builds and ordinary Beams.
    shell: cp /opt/app-config/app.json /home/beams/app.json

Full format reference, service options, and examples:
  https://goteleport.com/docs/beams/beams/#yaml-configuration-reference`

	phaseEditHelp = `
Examples:
  Open a build console to edit a Phase's saved interactive files:
  $ tsh phase edit dev-tools

  Start editing without opening a console:
  $ tsh phase edit dev-tools --no-console

  Replace the raw command file:
  $ tsh phase edit dev-tools --commands-file=build.yaml

  Replace a Phase's copied files using current local files:
  $ tsh phase edit app-files --copy=/opt/app/config:./config

  Edit using another Phase's snapshot with the same parent:
  $ tsh phase edit dev-tools --from=other-tools`

	phaseSnapshotHelp = `
Examples:
  Save changes from the current build console and keep working:
  $ tsh phase snapshot

  Save changes and end the current build:
  $ tsh phase snapshot --final

  Exclude changes outside /home and home paths reserved for rebuilds:
  $ tsh phase snapshot --force

  Save an existing snapshot to its Phase:
  $ tsh phase snapshot dev-tools --from='parent@sha256:<digest>'`

	ghHelp = `
Examples:
  Show the GitHub user:
  $ tsh gh -- api /user

  View a repository when more than one GitHub organization is available:
  $ tsh gh --github-org my-org -- repo view my-org/my-repo`
)
