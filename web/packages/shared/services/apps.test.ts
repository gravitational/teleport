/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { getAppProtocol, getAppUriScheme } from './apps';
import type { AppProtocol } from './types';

describe('getAppProtocol', () => {
  test.each<[string, AppProtocol]>([
    // TCP
    ['tcp://localhost:8080', 'TCP'],

    // MCP
    ['mcp+stdio://', 'MCP'],
    ['mcp+http://example.com/mcp', 'MCP'],
    ['mcp+sse+https://example.com/sse', 'MCP'],

    // HTTP (fallback/default)
    ['http://localhost:8080', 'HTTP'],
    ['https://localhost:8080', 'HTTP'],
    ['cloud://AWS', 'HTTP'],
  ])('%s is %s', (uri, expected) => {
    expect(getAppProtocol(uri)).toBe(expected);
  });
});

describe('getAppUriScheme', () => {
  test.each<[string, string]>([
    ['tcp://localhost:8080', 'tcp'],
    ['https://localhost:8080', 'https'],
    ['mcp+http://example.com/mcp', 'mcp+http'],
    ['', ''],
  ])('scheme from %s is %s', (uri, expected) => {
    expect(getAppUriScheme(uri)).toBe(expected);
  });
});
