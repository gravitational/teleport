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

import { resolve } from 'node:path';

import tsconfigPaths from 'vite-tsconfig-paths';

const rootDirectory = resolve(import.meta.dirname, '../../../..');

export function tsconfigPathsPlugin() {
  return tsconfigPaths({
    // Asking vite to crawl the root directory (by defining the `root` object, rather than `projects`) causes vite builds to fail
    // with a:
    //
    // "Error: ENOTDIR: not a directory, scandir '/go/src/github.com/gravitational/teleport/docker/ansible/rdir/rdir/rdir'""
    //
    // on a Debian GNU/Linux 10 (buster) (buildbox-node) Docker image running on an arm64 Macbook macOS 14.1.2. It's not clear why
    // this happens, however, defining the tsconfig file directly works around the issue.
    projects: [resolve(rootDirectory, 'tsconfig.json')],
  });
}
