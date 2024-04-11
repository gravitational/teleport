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
import packageJson from './package.json';

const rootDirectory = resolve(__dirname, '../../..');

// these dependencies don't play well unless they're externalized
// if Vite complains about a dependency, add it here
const externalizeDeps = ['strip-ansi', 'ansi-regex', 'd3-color'];

/**
 * Produces a base electron-vite config.
 * Provided paths are either relative or absolute, depending on the needs.
 * The paths to the source code are always absolute, Connect Enterprise can
 * provide its own implementation through `inputs` or keep using the OSS defaults.
 * The output paths, on the other hand, are always relative: both OSS and Enterprise
 * Connect must keep the output code in their respective directories.
 *
 * @param inputs - contains paths to the app entry points.
 */
export const makeConfig = (inputs: { rendererRoot: string }) =>
  defineConfig(env => {
    const tsConfigPathsPlugin = tsconfigPaths({
      projects: [resolve(rootDirectory, 'tsconfig.json')],
    });
    const commonPlugins = [
      externalizeDepsPlugin({
        exclude: externalizeDeps,
        // The `externalizeDepsPlugin` plugin is not able to detect dependencies
        // from Connect OSS package.json when it is run in the enterprise app context
        // (it simply reads the direct package.json content).
        // We have to provide them manually.
        include: Object.keys(packageJson.dependencies),
      }),
      tsConfigPathsPlugin,
    ];

    const config: UserConfig = {
      main: {
        build: {
          outDir: 'build/app/main',
          rollupOptions: {
            input: {
              index: resolvePathFromDirname('src/main.ts'),
              sharedProcess: resolvePathFromDirname(
                'src/sharedProcess/sharedProcess.ts'
              ),
              agentCleanupDaemon: resolvePathFromDirname(
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
          outDir: 'build/app/preload',
          rollupOptions: {
            input: {
              index: resolvePathFromDirname('src/preload.ts'),
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
        root: inputs.rendererRoot,
        build: {
          outDir: 'build/app/renderer',
          rollupOptions: {
            input: {
              index: 'index.html',
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

export default makeConfig({ rendererRoot: __dirname });

function manualChunks(id: string) {
  for (const dep of externalizeDeps) {
    if (id.includes(dep)) {
      return dep;
    }
  }
}

/**
 * Returns an absolute path that is relative to the current script location.
 * This allows both OSS and Enterprise Connect point to the same OSS files.
 */
function resolvePathFromDirname(path: string): string {
  return resolve(__dirname, path);
}
