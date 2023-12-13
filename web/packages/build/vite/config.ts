/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { existsSync, readFileSync } from 'fs';
import { resolve } from 'path';

import { defineConfig } from 'vite';
import { visualizer } from 'rollup-plugin-visualizer';

import react from '@vitejs/plugin-react-swc';
import tsconfigPaths from 'vite-tsconfig-paths';

import { htmlPlugin, transformPlugin } from './html';
import { getStyledComponentsConfig } from './styled';

import type { UserConfig } from 'vite';

const DEFAULT_PROXY_TARGET = '127.0.0.1:3080';

export function createViteConfig(
  rootDirectory: string,
  outputDirectory: string
) {
  return defineConfig(({ mode }) => {
    let target = resolveTargetURL(process.env.PROXY_TARGET);

    if (mode === 'development') {
      if (process.env.PROXY_TARGET) {
        console.log(
          `  \x1b[32m✔ Proxying requests to ${target.toString()}\x1b[0m`
        );
      } else {
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
      },
      plugins: [
        react({
          plugins: [
            ['@swc/plugin-styled-components', getStyledComponentsConfig(mode)],
          ],
        }),
        tsconfigPaths({
          // Asking vite to crawl the root directory (by defining the `root` object, rather than `projects`) causes vite builds to fail
          // with a:
          //
          // "Error: ENOTDIR: not a directory, scandir '/go/src/github.com/gravitational/teleport/docker/ansible/rdir/rdir/rdir'""
          //
          // on a Debian GNU/Linux 10 (buster) (buildbox-node) Docker image running on an arm64 Macbook macOS 14.1.2. It's not clear why
          // this happens, however defining the tsconfig file directly works around the issue.
          projects: [resolve(rootDirectory, 'tsconfig.json')],
        }),
        transformPlugin(),
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
