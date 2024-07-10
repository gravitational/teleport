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

import { existsSync, readFileSync } from 'fs';
import { resolve } from 'path';

import { defineConfig } from 'vite';
import { visualizer } from 'rollup-plugin-visualizer';
import wasm from 'vite-plugin-wasm';

import { htmlPlugin, transformPlugin } from './html';
import { generateAppHashFile } from './apphash';
import { reactPlugin } from './react.mjs';
import { tsconfigPathsPlugin } from './tsconfigPaths.mjs';

import type { UserConfig } from 'vite';

const DEFAULT_PROXY_TARGET = '127.0.0.1:3080';
const ENTRY_FILE_NAME = 'app/app.js';

export function createViteConfig(
  rootDirectory: string,
  outputDirectory: string
) {
  return defineConfig(({ mode }) => {
    let target = resolveTargetURL(process.env.PROXY_TARGET);

    if (mode === 'development') {
      if (process.env.PROXY_TARGET) {
        // eslint-disable-next-line no-console
        console.log(
          `  \x1b[32m✔ Proxying requests to ${target.toString()}\x1b[0m`
        );
      } else {
        // eslint-disable-next-line no-console
        console.warn(
          `  \x1b[33m⚠ PROXY_TARGET was not set, defaulting to ${DEFAULT_PROXY_TARGET}\x1b[0m`
        );

        target = DEFAULT_PROXY_TARGET;
      }
    }

    const config: UserConfig = {
      clearScreen: false,
      server: {
        fs: {
          allow: [rootDirectory, '.'],
        },
        host: '0.0.0.0',
        port: 3000,
      },
      build: {
        outDir: outputDirectory,
        assetsDir: 'app',
        emptyOutDir: true,
        rollupOptions: {
          output: {
            // removes hashing from our entry point file.
            entryFileNames: ENTRY_FILE_NAME,
            // the telemetry bundle breaks any websocket connections if included in the bundle. We will leave this file out of the bundle but without hashing so it is still discoverable.
            // TODO (avatus): find out why this breaks websocket connectivity and unchunk
            chunkFileNames: 'app/[name].js',
            // this will remove hashing from asset (non-js) files.
            assetFileNames: `app/[name].[ext]`,
          },
        },
      },
      plugins: [
        reactPlugin(mode),
        tsconfigPathsPlugin(),
        transformPlugin(),
        generateAppHashFile(outputDirectory, ENTRY_FILE_NAME),
        wasm(),
      ],
      define: {
        'process.env': { NODE_ENV: process.env.NODE_ENV },
      },
    };

    if (process.env.VITE_ANALYZE_BUNDLE) {
      config.plugins.push(visualizer());
    }

    if (mode === 'production') {
      config.base = '/web';
    } else {
      config.plugins.push(htmlPlugin(target));
      // siteName matches everything between the slashes.
      const siteName = '([^\\/]+)';

      config.server.proxy = {
        // The format of the regex needs to assume that the slashes are escaped, for example:
        // \/v1\/webapi\/sites\/:site\/connect
        [`^\\/v1\\/webapi\\/sites\\/${siteName}\\/connect`]: {
          target: `wss://${target}`,
          changeOrigin: false,
          secure: false,
          ws: true,
        },
        // /webapi/sites/:site/desktops/:desktopName/connect
        [`^\\/v1\\/webapi\\/sites\\/${siteName}\\/desktops\\/${siteName}\\/connect`]:
          {
            target: `wss://${target}`,
            changeOrigin: false,
            secure: false,
            ws: true,
          },
        // /webapi/sites/:site/kube/exec
        [`^\\/v1\\/webapi\\/sites\\/${siteName}\\/kube/exec`]: {
          target: `wss://${target}`,
          changeOrigin: false,
          secure: false,
          ws: true,
        },
        // /webapi/sites/:site/desktopplayback/:sid
        '^\\/v1\\/webapi\\/sites\\/(.*?)\\/desktopplayback\\/(.*?)': {
          target: `wss://${target}`,
          changeOrigin: false,
          secure: false,
          ws: true,
        },
        '^\\/v1\\/webapi\\/assistant\\/(.*?)': {
          target: `https://${target}`,
          changeOrigin: false,
          secure: false,
        },
        [`^\\/v1\\/webapi\\/sites\\/${siteName}\\/assistant`]: {
          target: `wss://${target}`,
          changeOrigin: false,
          secure: false,
          ws: true,
        },
        '^\\/v1\\/webapi\\/command\\/(.*?)/execute': {
          target: `wss://${target}`,
          changeOrigin: false,
          secure: false,
          ws: true,
        },
        '/web/config.js': {
          target: `https://${target}`,
          changeOrigin: true,
          secure: false,
        },
        '/v1': {
          target: `https://${target}`,
          changeOrigin: true,
          secure: false,
        },
      };

      if (process.env.VITE_HTTPS_KEY && process.env.VITE_HTTPS_CERT) {
        config.server.https = {
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

        config.server.https = {
          key: readFileSync(keyPath),
          cert: readFileSync(certPath),
        };
      }
    }

    return config;
  });
}

function resolveTargetURL(url: string) {
  if (!url) {
    return;
  }

  const target = url.startsWith('https') ? url : `https://${url}`;

  const parsed = new URL(target);

  return parsed.host;
}
