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
import wasm from 'vite-plugin-wasm';

import { htmlPlugin, transformPlugin } from './html';
import { getStyledComponentsConfig } from './styled';

import type { UserConfig } from 'vite';

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
          '  \x1b[33m⚠ PROXY_TARGET was not set, defaulting to localhost:3080\x1b[0m'
        );

        target = 'localhost:3080';
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
          root: rootDirectory,
        }),
        transformPlugin(),
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

      config.server.proxy = {
        '^\\/v1\\/webapi\\/sites\\/(.*?)\\/connect': {
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
        '^\\/v1\\/webapi\\/sites\\/(.*?)\\/assistant': {
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
