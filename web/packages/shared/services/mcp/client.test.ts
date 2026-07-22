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

import {
  generateClaudeDesktopConfigForApp,
  generateInstallLinksForApp,
} from 'shared/services/mcp/client';

describe('generateClaudeDesktopConfigForApp', () => {
  it('generates the correct config JSON', () => {
    const outputJSON = generateClaudeDesktopConfigForApp('demo-test');

    expect(outputJSON).toBe(`{
  "mcpServers": {
    "teleport-mcp-demo-test": {
      "command": "tsh",
      "args": [
        "mcp",
        "connect",
        "demo-test"
      ]
    }
  }
}`);
  });
});

describe('generateInstallLinksForApp', () => {
  it('generates the correct links', () => {
    const links = generateInstallLinksForApp('demo-test');
    expect(links).toEqual({
      cursor:
        'cursor://anysphere.cursor-deeplink/mcp/install?name=teleport-mcp-demo-test&config=eyJjb21tYW5kIjoidHNoIiwiYXJncyI6WyJtY3AiLCJjb25uZWN0IiwiZGVtby10ZXN0Il19',
      vscode:
        'vscode:mcp/install?%7B%22name%22%3A%22teleport-mcp-demo-test%22%2C%22command%22%3A%22tsh%22%2C%22args%22%3A%5B%22mcp%22%2C%22connect%22%2C%22demo-test%22%5D%7D',
      vscodeInsiders:
        'vscode-insiders:mcp/install?%7B%22name%22%3A%22teleport-mcp-demo-test%22%2C%22command%22%3A%22tsh%22%2C%22args%22%3A%5B%22mcp%22%2C%22connect%22%2C%22demo-test%22%5D%7D',
    });
  });
});
