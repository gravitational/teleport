/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

export type MCPServerConfig = {
  command: string;
  args: string[];
};

function mcpServerNameForApp(appName: string): string {
  return `teleport-mcp-${appName}`;
}

function mcpServerConfigForApp(appName: string): MCPServerConfig {
  return {
    // TODO(greedy52) we might need different command path and TELEPORT_HOME
    // env var for Teleport Connect.
    command: 'tsh',
    args: ['mcp', 'connect', appName],
  };
}

/**
 * generateClaudeDesktopConfigForApp generates a prettified JSON config with
 * details to launch the MCP server app with tsh in Claude Desktop format.
 */
export function generateClaudeDesktopConfigForApp(appName: string): string {
  const claudeConfig = {
    mcpServers: {
      [mcpServerNameForApp(appName)]: mcpServerConfigForApp(appName),
    },
  };
  return JSON.stringify(claudeConfig, null, 2);
}

export type InstallLinks = {
  cursor: string;
  vscode: string;
  vscodeInsiders: string;
};

/**
 * generateInstallLinksForApp generates links that can be used to install the MCP
 * server app (that runs via tsh) for various MCP clients like cursor.
 */
export function generateInstallLinksForApp(appName: string): InstallLinks {
  const name = mcpServerNameForApp(appName);
  const config = mcpServerConfigForApp(appName);

  // Cursor Deeplink
  // https://docs.cursor.com/tools/developers
  const cursorLink = new URL('cursor://anysphere.cursor-deeplink/mcp/install');
  cursorLink.searchParams.set('name', name);
  cursorLink.searchParams.set('config', btoa(JSON.stringify(config)));

  // VSCode
  // https://code.visualstudio.com/docs/copilot/chat/mcp-servers#_url-handler
  const vscodeEncodedConfig = encodeURIComponent(
    JSON.stringify({
      name,
      ...config,
    })
  );
  return {
    cursor: cursorLink.toString(),
    vscode: `vscode:mcp/install?${vscodeEncodedConfig}`,
    vscodeInsiders: `vscode-insiders:mcp/install?${vscodeEncodedConfig}`,
  };
}
