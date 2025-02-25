/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

/**
 * This needs to be an .mjs file,
 * because Vite can't import .ts files from a workspace out of the box.
 * https://github.com/vitejs/vite/issues/5370
 */

import path from 'node:path';

import tsconfigPaths from 'vite-tsconfig-paths';

const rootDirectory = path.resolve(import.meta.dirname, '../../../..');

export function tsconfigPathsPlugin() {
  return tsconfigPaths({
    root: rootDirectory,
  });
}
