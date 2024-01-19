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

import { resolve } from 'path';

import { existsSync, readFileSync } from 'fs';

import react from '@vitejs/plugin-react-swc';
import tsconfigPaths from 'vite-tsconfig-paths';
import { defineConfig, externalizeDepsPlugin, UserConfig } from 'electron-vite';

import { cspPlugin } from '../build/vite/csp';

import { getStyledComponentsConfig } from '../build/vite/styled';

import { getConnectCsp } from './csp';

const rootDirectory = resolve(__dirname, '../../..');
const outputDirectory = resolve(__dirname, 'build');

// these dependencies don't play well unless they're externalized
// if Vite complains about a dependency, add it here
const externalizeDeps = ['strip-ansi', 'ansi-regex', 'd3-color'];

const config = defineConfig(env => {
  const tsConfigPathsPlugin = tsconfigPaths({
    projects: [resolve(rootDirectory, 'tsconfig.json')],
  });

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
          onwarn,
          output: {
            manualChunks,
          },
        },
      },
      plugins: commonPlugins,
    },
    preload: {
      build: {
        outDir: resolve(outputDirectory, 'preload'),
        rollupOptions: {
          input: {
            index: resolve(__dirname, 'src/preload.ts'),
          },
          onwarn,
          output: {
            manualChunks,
          },
        },
      },
      plugins: commonPlugins,
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
        host: '0.0.0.0',
        port: 8080,
        fs: {
          allow: [rootDirectory, '.'],
        },
      },
      plugins: [
        react({
          plugins: [
            [
              '@swc/plugin-styled-components',
              getStyledComponentsConfig(env.mode),
            ],
          ],
        }),
        cspPlugin(getConnectCsp(env.mode === 'development')),
        tsConfigPathsPlugin,
      ],
      define: {
        'process.env': { NODE_ENV: process.env.NODE_ENV },
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

function onwarn(warning, warn) {
  // ignore Vite complaining about protobufs using eval
  if (warning.code === 'EVAL') {
    return;
  }

  warn(warning);
}

function manualChunks(id: string) {
  for (const dep of externalizeDeps) {
    if (id.includes(dep)) {
      return dep;
    }
  }
}
