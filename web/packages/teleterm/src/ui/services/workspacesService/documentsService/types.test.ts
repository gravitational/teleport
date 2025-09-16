/**
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

import {
  DocumentTshNodeWithLoginHost,
  DocumentTshNodeWithServerId,
  isDocumentTshNodeWithLoginHost,
  isDocumentTshNodeWithServerId,
} from './types';

const docWithServerId: DocumentTshNodeWithServerId = {
  kind: 'doc.terminal_tsh_node',
  uri: '/docs/123',
  title: '',
  status: '',
  serverId: 'bed30649-3af5-40f1-a832-54ff4adcca41',
  serverUri: `/clusters/root/servers/bed30649-3af5-40f1-a832-54ff4adcca41`,
  rootClusterId: 'test',
  leafClusterId: undefined,
  login: 'user',
  origin: 'resource_table',
};
// eslint-disable-next-line @typescript-eslint/no-unused-vars
const { serverId, serverUri, login, ...rest } = docWithServerId;
const docWithLoginHost: DocumentTshNodeWithLoginHost = {
  ...rest,
  loginHost: 'user@bar',
};

// Testing type guards because TypeScript doesn't guarantee soundness inside them.
test('isDocumentTshNodeWithServerId returns true for DocumentTshNode with ServerId', () => {
  expect(isDocumentTshNodeWithServerId(docWithServerId)).toBe(true);
});

test('isDocumentTshNodeWithServerId returns false for DocumentTshNode with LoginHost', () => {
  expect(isDocumentTshNodeWithServerId(docWithLoginHost)).toBe(false);
});

test('isDocumentTshNodeWithLoginHost returns true for DocumentTshNode with LoginHost', () => {
  expect(isDocumentTshNodeWithLoginHost(docWithLoginHost)).toBe(true);
});

test('isDocumentTshNodeWithLoginHost returns false for DocumentTshNode with ServerId', () => {
  expect(isDocumentTshNodeWithLoginHost(docWithServerId)).toBe(false);
});
