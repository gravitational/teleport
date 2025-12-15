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

const path = require('path');

module.exports = {
  testEnvironment: path.join(__dirname, 'jest-environment-patched-jsdom.js'),
  moduleNameMapper: {
    // mock all imports to asset files
    '\\.(css|scss|stylesheet)$': path.join(__dirname, 'mockStyles.js'),
    '\\.(png|svg|svg\\?no-inline|yaml|yaml\\?raw)$': path.join(
      __dirname,
      'mockFiles.js'
    ),
    '^shared/(.*)$': '<rootDir>/web/packages/shared/$1',
    '^design($|/.*)': '<rootDir>/web/packages/design/src/$1',
    '^teleport($|/.*)': '<rootDir>/web/packages/teleport/src/$1',
    '^teleterm($|/.*)': '<rootDir>/web/packages/teleterm/src/$1',
    '^e-teleport/(.*)$': '<rootDir>/e/web/teleport/src/$1',
    '^gen-proto-js/(.*)$': '<rootDir>/gen/proto/js/$1',
    '^gen-proto-ts/(.*)$': '<rootDir>/gen/proto/ts/$1',
  },
  // Keep pre-v29 snapshot format to avoid existing snapshots breaking.
  // https://jestjs.io/docs/upgrading-to-jest29#snapshot-format
  snapshotFormat: {
    escapeString: true,
    printBasicPrototype: true,
  },
  showSeed: true,
};
