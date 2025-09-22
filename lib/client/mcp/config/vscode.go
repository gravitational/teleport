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

package config

const (
	// vsCodeServersKey defines the JSON key used by VSCode to store MCP servers
	// config.
	//
	// https://code.visualstudio.com/docs/copilot/chat/mcp-servers#_manage-mcp-servers
	vsCodeServersKey = "servers"
	// vsCodeProjectDir defines the VSCode project directory, where the MCP
	// configuration is located.
	//
	// https://code.visualstudio.com/docs/copilot/chat/mcp-servers#_manage-mcp-servers
	vsCodeProjectDir = ".vscode"
	// claudeCodeFileName defines the Claude Code MCP servers file.
	//
	// https://docs.claude.com/en/docs/claude-code/mcp#project-scope
	claudeCodeFileName = ".mcp.json"
)
