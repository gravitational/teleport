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

const crypt = require('crypto');
const path = require('path');

const failOnConsole = require('jest-fail-on-console');

let entFailOnConsoleIgnoreList = [];
try {
  entFailOnConsoleIgnoreList = require('../../../e/web/testsWithIgnoredConsole');
} catch (err) {
  // Ignore errors related to teleport.e not being present. This allows OSS users and OSS CI to run
  // tests without teleport.e.
  if (err['code'] !== 'MODULE_NOT_FOUND') {
    throw err;
  }
}

Object.defineProperty(globalThis, 'crypto', {
  value: {
    randomUUID: () => crypt.randomUUID(),
  },
});

global.ResizeObserver = jest.fn().mockImplementation(() => ({
  observe: jest.fn(),
  unobserve: jest.fn(),
  disconnect: jest.fn(),
}));

const rootDir = path.join(__dirname, '..', '..', '..');
// Do not add new paths to this list, instead fix the underlying problem which causes console.error
// or console.warn to be used.
//
// If the test is expected to use either of those console functions, follow the advice from the
// error message.
const failOnConsoleIgnoreList = new Set([
  'web/packages/design/src/utils/match/matchers.test.ts',
  'web/packages/shared/components/TextEditor/TextEditor.test.tsx',
  'web/packages/teleport/src/components/BannerList/useAlerts.test.tsx',
  'web/packages/teleport/src/Navigation/NavigationItem.test.tsx',
  'web/packages/teleterm/src/ui/TabHost/TabHost.test.tsx',
  // As of the parent commit (708dac8e0d0), the tests below are flakes.
  // https://github.com/gravitational/teleport/pull/41252#discussion_r1595036569
  'web/packages/teleport/src/Console/DocumentNodes/DocumentNodes.story.test.tsx',
  'web/packages/teleport/src/Recordings/Recordings.story.test.tsx',
  'web/packages/teleport/src/Audit/Audit.story.test.tsx',
  ...entFailOnConsoleIgnoreList,
]);
failOnConsole({
  skipTest: ({ testPath }) => {
    const relativeTestPath = path.relative(rootDir, testPath);
    return failOnConsoleIgnoreList.has(relativeTestPath);
  },
});
