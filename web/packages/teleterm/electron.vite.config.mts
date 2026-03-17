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

import { createRequire } from 'node:module';
import { builtinModules } from 'node:module';
import path from 'node:path';

import { defineConfig, UserConfig } from 'electron-vite';
import type { RolldownOptions } from 'rolldown';
import type { Plugin } from 'vite';

import { reactPlugin } from '@gravitational/build/vite/react.mjs';

import { getConnectCsp } from './csp';

const rootDirectory = path.resolve(__dirname, '../../..');
const outputDirectory = path.resolve(__dirname, 'build', 'app');

// these dependencies don't play well unless they're externalized
// if Vite complains about a dependency, add it here
const externalizeDeps = ['strip-ansi', 'ansi-regex', 'd3-color'];

// electron-vite's externalizeDepsPlugin sets build.rollupOptions.external, which
// Vite 8 ignores (it uses rolldownOptions).
// TODO(ryan): Remove this once electron-vite supports Vite 8.
//
// electron-vite externalizes electron, Node.js built-in modules, and package.json
// dependencies for main and preload, but bundles everything for the renderer.
// See https://electron-vite.org/guide/dependency-handling.
const pkg = createRequire(import.meta.url)('./package.json');
const deps = Object.keys(pkg.dependencies || {}).filter(
  dep => !externalizeDeps.includes(dep)
);

const commonRolldownOptions: RolldownOptions = {
  onLog(level, log, defaultHandler) {
    // Suppress direct eval warning from @protobufjs/inquire.
    // The eval is intentional (to call require without bundler detection) and patching
    // it to indirect eval would break Electron's module-scoped require.
    if (log.code === 'EVAL' && log.id?.includes('@protobufjs/inquire')) {
      return;
    }

    defaultHandler(level, log);
  },
};

// Main and preload run in Node.js, so we externalize electron, Node.js built-in
// modules, and package.json dependencies (they'll be included during packaging).
const nodeExternalOptions: RolldownOptions = {
  external: [
    'electron',
    /^electron\/.+/,
    ...builtinModules.flatMap(m => [m, `node:${m}`]),
    ...deps,
    new RegExp(`^(${deps.join('|')})/.+`),
  ],
};

const config = defineConfig(env => {
  const config: UserConfig = {
    main: {
      resolve: {
        tsconfigPaths: true,
      },
      build: {
        outDir: path.resolve(outputDirectory, 'main'),
        rolldownOptions: {
          ...commonRolldownOptions,
          ...nodeExternalOptions,
          input: {
            index: path.resolve(__dirname, 'src/main.ts'),
            sharedProcess: path.resolve(
              __dirname,
              'src/sharedProcess/sharedProcess.ts'
            ),
            agentCleanupDaemon: path.resolve(
              __dirname,
              'src/agentCleanupDaemon/agentCleanupDaemon.js'
            ),
          },
          output: {
            format: 'cjs',
            manualChunks,
          },
        },
      },
      define: {
        // It's not common to pre-process Node code with NODE_ENV, but this is what our Webpack
        // config used to do, so for compatibility purposes we kept the Vite config this way.
        //
        // If we were to get rid of this, we'd somehow need to set NODE_ENV when the packaged app
        // gets started.
        'process.env.NODE_ENV': JSON.stringify(process.env.NODE_ENV),
      },
    },
    preload: {
      resolve: {
        tsconfigPaths: true,
      },
      build: {
        outDir: path.resolve(outputDirectory, 'preload'),
        rolldownOptions: {
          ...commonRolldownOptions,
          ...nodeExternalOptions,
          input: {
            index: path.resolve(__dirname, 'src/preload.ts'),
          },
          output: {
            format: 'cjs',
            manualChunks,
          },
        },
      },
      define: {
        // Preload is also mean to be run by Node, see the comment for define under main.
        'process.env.NODE_ENV': JSON.stringify(process.env.NODE_ENV),
      },
    },
    renderer: {
      resolve: {
        tsconfigPaths: true,
      },
      assetsInclude: ['**/shared/libs/ironrdp/**/*.wasm'],
      root: '.',
      build: {
        outDir: path.resolve(outputDirectory, 'renderer'),
        rolldownOptions: {
          ...commonRolldownOptions,
          input: {
            index: path.resolve(__dirname, 'index.html'),
          },
        },
      },
      server: {
        host: 'localhost',
        port: 8080,
        fs: {
          allow: [rootDirectory, '.'],
        },
      },
      plugins: [
        reactPlugin(env.mode),
        cspPlugin(getConnectCsp(env.mode === 'development')),
        {
          // The IronRDP wasm module is embedded into the renderer app earlier in the build.
          // Exclude it here, otherwise rollup still emits it as a static asset by default.
          name: 'drop-wasm-assets',
          generateBundle(_, bundle) {
            for (const file of Object.keys(bundle)) {
              if (file.endsWith('.wasm')) {
                delete bundle[file];
              }
            }
          },
        },
      ],
      define: {
        'process.env.NODE_ENV': JSON.stringify(process.env.NODE_ENV),
      },
    },
  };

  return config;
});

export { config as default };

function manualChunks(id: string) {
  for (const dep of externalizeDeps) {
    if (id.includes(dep)) {
      return dep;
    }
  }
}

function cspPlugin(csp: string): Plugin {
  return {
    name: 'teleport-connect-html-plugin',
    transformIndexHtml(html) {
      return {
        html,
        tags: [
          {
            tag: 'meta',
            attrs: {
              'http-equiv': 'Content-Security-Policy',
              content: csp,
            },
          },
        ],
      };
    },
  };
}
