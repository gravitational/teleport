/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import {
  isDocumentTshNodeWithServerId,
  isDocumentTshNodeWithLoginHost,
  DocumentTshNodeWithServerId,
  DocumentTshNodeWithLoginHost,
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
