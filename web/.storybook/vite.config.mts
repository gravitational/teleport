import fs from 'node:fs';
import { createRequire } from 'node:module';

import { defineConfig, type Plugin } from 'vite';

import { reactPlugin } from '@gravitational/build/vite/react.mjs';

export default defineConfig(({ mode }) => ({
  resolve: {
    tsconfigPaths: true,
  },
  plugins: [reactPlugin(mode), serveStorybookMockerRuntime()],
  assetsInclude: ['**/shared/libs/ironrdp/**/*.wasm'],
}));

/**
 * Storybook's mocker runtime is a small JavaScript file that Storybook injects into the
 * preview iframe to enable its mocking features.
 *
 * When pnpm's virtual store is used, this file does not load (404), so we serve it ourselves.
 */
function serveStorybookMockerRuntime(): Plugin {
  const ENTRY_PATH = '/vite-inject-mocker-entry.js';
  const require = createRequire(import.meta.url);

  let runtime: string | null = null;

  return {
    name: 'teleport:storybook-mocker-runtime-dev',
    enforce: 'pre',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        if (req.url === ENTRY_PATH) {
          if (runtime === null) {
            runtime = fs.readFileSync(
              require.resolve('storybook/internal/mocking-utils/mocker-runtime'),
              'utf-8'
            );
          }

          res.setHeader('Content-Type', 'application/javascript');
          res.end(runtime);

          return;
        }

        next();
      });
    },
  };
}
