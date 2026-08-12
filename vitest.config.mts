/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import fs from 'node:fs';

import { createVitestConfig } from './web/packages/build/vitest/config';

// Gradual migration: Vitest runs only *.vitest.{ts,tsx}; everything still on
// *.test.{ts,tsx} keeps running under Jest.
const include = ['web/**/*.vitest.{ts,tsx}'];

// Include enterprise tests when the e/ directory is present.
if (fs.existsSync('e/web')) {
  include.push('e/web/**/*.vitest.{ts,tsx}');
}

export default createVitestConfig(include);
