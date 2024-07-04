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

import { resolve } from 'node:path';
import { existsSync, readFileSync } from 'node:fs';

import { defineConfig, externalizeDepsPlugin, UserConfig } from 'electron-vite';

import { reactPlugin } from '@gravitational/build/vite/react.mjs';
import { tsconfigPathsPlugin } from '@gravitational/build/vite/tsconfigPaths.mjs';

import { getConnectCsp } from './csp';

import type { Plugin } from 'vite';

const rootDirectory = resolve(__dirname, '../../..');
const outputDirectory = resolve(__dirname, 'build', 'app');

// these dependencies don't play well unless they're externalized
// if Vite complains about a dependency, add it here
const externalizeDeps = ['strip-ansi', 'ansi-regex', 'd3-color'];

const config = defineConfig(env => {
  const tsConfigPathsPlugin = tsconfigPathsPlugin();

  const commonPlugins = [
    externalizeDepsPlugin({ exclude: externalizeDeps }),
    tsConfigPathsPlugin,
  ];

  const config: UserConfig = {
    main: {
      build: {
        outDir: resolve(outputDirectory, 'main'),
        rollupOptions: {
          input: {
            index: resolve(__dirname, 'src/main.ts'),
            sharedProcess: resolve(
              __dirname,
              'src/sharedProcess/sharedProcess.ts'
            ),
            agentCleanupDaemon: resolve(
              __dirname,
              'src/agentCleanupDaemon/agentCleanupDaemon.js'
            ),
          },
          output: {
            manualChunks,
          },
        },
      },
      plugins: commonPlugins,
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
      build: {
        outDir: resolve(outputDirectory, 'preload'),
        rollupOptions: {
          input: {
            index: resolve(__dirname, 'src/preload.ts'),
          },
          output: {
            manualChunks,
          },
        },
      },
      plugins: commonPlugins,
      define: {
        // Preload is also mean to be run by Node, see the comment for define under main.
        'process.env.NODE_ENV': JSON.stringify(process.env.NODE_ENV),
      },
    },
    renderer: {
      root: '.',
      build: {
        outDir: resolve(outputDirectory, 'renderer'),
        rollupOptions: {
          input: {
            index: resolve(__dirname, 'index.html'),
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
        tsConfigPathsPlugin,
      ],
      define: {
        'process.env.NODE_ENV': JSON.stringify(process.env.NODE_ENV),
      },
    },
  };

  if (env.mode === 'development') {
    if (process.env.VITE_HTTPS_KEY && process.env.VITE_HTTPS_CERT) {
      config.renderer.server.https = {
        key: readFileSync(process.env.VITE_HTTPS_KEY),
        cert: readFileSync(process.env.VITE_HTTPS_CERT),
      };
    } else {
      const certsDirectory = resolve(rootDirectory, 'web/certs');

      if (!existsSync(certsDirectory)) {
        throw new Error(
          'Could not find SSL certificates. Please follow web/README.md to generate certificates.'
        );
      }

      const keyPath = resolve(certsDirectory, 'server.key');
      const certPath = resolve(certsDirectory, 'server.crt');

      config.renderer.server.https = {
        key: readFileSync(keyPath),
        cert: readFileSync(certPath),
      };
    }
  }

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
