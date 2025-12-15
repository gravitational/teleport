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

import 'whatwg-fetch';

import crypto from 'node:crypto';
import path from 'node:path';

import failOnConsole from 'jest-fail-on-console';
import { configMocks } from 'jsdom-testing-mocks';
import { act } from 'react';

configMocks({ act });

let entFailOnConsoleIgnoreList = [];
try {
  // Cannot do `await import` yet here.
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  entFailOnConsoleIgnoreList = require('../../../../e/web/testsWithIgnoredConsole');
} catch (err) {
  // Ignore errors related to teleport.e not being present. This allows OSS users and OSS CI to run
  // tests without teleport.e.
  if (err['code'] !== 'MODULE_NOT_FOUND') {
    throw err;
  }
}

Object.defineProperty(globalThis, 'crypto', {
  value: {
    randomUUID: () => crypto.randomUUID(),
  },
});

const rootDir = path.join(__dirname, '..', '..', '..', '..');
// Do not add new paths to this list, instead fix the underlying problem which causes console.error
// or console.warn to be used.
//
// If the test is expected to use either of those console functions, follow the advice from the
// error message.
const failOnConsoleIgnoreList = new Set([
  ...entFailOnConsoleIgnoreList,
  'web/packages/design/src/utils/match/matchers.test.ts',
  'web/packages/shared/components/TextEditor/TextEditor.test.tsx',
  'web/packages/teleport/src/components/BannerList/useAlerts.test.tsx',
]);

// A list of allowed messages, for expected messages that shouldn't fail the build (e.g., warnings
// about deprecated functions from 3rd-party libraries).
const failOnConsoleAllowedMessages = [];

failOnConsole({
  skipTest: ({ testPath }) => {
    const relativeTestPath = path.relative(rootDir, testPath);
    return failOnConsoleIgnoreList.has(relativeTestPath);
  },
  allowMessage: (message: string) =>
    failOnConsoleAllowedMessages.some(allowedMessageFragment =>
      message.includes(allowedMessageFragment)
    ),
});
