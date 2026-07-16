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

// Must precede the imports: vmThreads VM contexts capture TZ at creation time.
process.env.TZ = 'UTC';

import { type Plugin, type ViteUserConfig, defineConfig } from 'vitest/config';

import { reactPlugin } from '../vite/react.mjs';

// Stubs assets that aren't real modules under test (images, YAML, and the
// ironrdp ?inline wasm, whose client is mocked by the tests that use it).
function assetStubPlugin(): Plugin {
  return {
    name: 'vitest-asset-stub',
    transform(_code, id) {
      if (/\.wasm(\?.*)?$/.test(id)) {
        return { code: `export default new Uint8Array();`, map: null };
      }
      if (/\.(png|svg|yaml)(\?.*)?$/.test(id)) {
        return { code: `export default 'file_stub';`, map: null };
      }
    },
  };
}

const standardExclude = ['**/e2e/**', '**/node_modules/**', '**/dist/**'];

export function createVitestConfig(testInclude: string[]): ViteUserConfig {
  return defineConfig({
    plugins: [reactPlugin('test'), assetStubPlugin()],
    resolve: { tsconfigPaths: true },
    test: {
      // Only for the dual-runner helpers (enableMswServer, trackingTester) that
      // call bare beforeAll/afterEach/expect and can't import from vitest without
      // breaking jest. Test files import from vitest directly.
      globals: true,
      pool: 'vmThreads',
      include: testInclude,
      exclude: standardExclude,
      // happy-dom resolves the design system's modern CSS that jsdom's parser drops (breaking toHaveStyle).
      environment: 'happy-dom',
      environmentOptions: {
        happyDOM: { url: 'http://localhost' },
        jsdom: { url: 'http://localhost' },
      },
      setupFiles: ['./web/packages/build/vitest/setup.ts'],
      css: false,
    },
  });
}
