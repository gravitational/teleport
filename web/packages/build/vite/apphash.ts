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

import { createHash } from 'crypto';
import { writeFileSync } from 'fs';
import { resolve } from 'path';

// this plugin is used to generate a file containing the hash of the app.js
// bundle. Because we omit the hash from the filename, we don't have access to it
// in the build chain. We generate a hash using the same methods used when files
// passed by default to `augmentChunkHash` (hash.update).
// https://rollupjs.org/plugin-development/#augmentchunkhash
export function generateAppHashFile(outputDir: string, entryFilename: string) {
  return {
    name: 'app-hash-plugin',
    generateBundle(_, bundle) {
      // bundle is OutputChunk | OutputAsset. These types aren't exported
      // by vite but by rollup, which isn't directly in our bundle so we
      // will use `any` instead of installing rollup
      // https://rollupjs.org/plugin-development/#generatebundle
      const { code } = bundle[entryFilename] as any;
      if (code) {
        const hash = createHash('sha256').update(code).digest('base64');
        writeFileSync(resolve(outputDir, 'apphash'), hash);
      }
    },
  };
}
