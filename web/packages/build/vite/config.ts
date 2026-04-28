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

import { visualizer } from 'rollup-plugin-visualizer';
import { defineConfig, type ProxyOptions, type UserConfig } from 'vite';
import { compression } from 'vite-plugin-compression2';
import wasm from 'vite-plugin-wasm';

import { generateAppHashFile } from './apphash';
import { guardWasmPlugin } from './guard-wasm';
import { htmlPlugin, transformPlugin } from './html';
import { reactPlugin } from './react.mjs';

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
      cacheDir: process.env.VITE_CACHE_DIR,
      server: {
        allowedHosts: resolveAllowedHosts(target),
        fs: {
          allow: [rootDirectory, '.'],
        },
        host: '0.0.0.0',
        port: 3000,
      },
      resolve: {
        tsconfigPaths: true,
      },
      optimizeDeps: {
        // Exclude from pre-bundling so the guard-wasm-globals plugin can transform the
        // module during dev.
        exclude: ['@xterm/addon-image'],
      },
      build: {
        outDir: outputDirectory,
        assetsDir: 'app',
        emptyOutDir: true,
        reportCompressedSize: false,
        rolldownOptions: {
          checks: {
            // We don't really need rolldown to complain about react/assets/wasm taking a "long"
            // time - the entire build takes ~7s with compression, which is plenty fast.
            pluginTimings: false,
          },
          onLog(level, log, defaultHandler) {
            // Suppress direct eval warning from @protobufjs/inquire.
            // The eval is intentional (to call require without bundler detection) and patching
            // it to indirect eval would break Electron's module-scoped require.
            if (
              log.code === 'EVAL' &&
              log.id?.includes('@protobufjs/inquire')
            ) {
              return;
            }

            defaultHandler(level, log);
          },
          output: {
            // removes hashing from our entry point file.
            entryFileNames: ENTRY_FILE_NAME,
            // the telemetry bundle breaks any websocket connections if included in the bundle. We will leave this file out of the bundle but without hashing so it is still discoverable.
            // TODO (avatus): find out why this breaks websocket connectivity and unchunk
            chunkFileNames: 'app/[name].js',
            // this will remove hashing from asset (non-js) files.
            assetFileNames: `app/[name].[ext]`,
          },
          plugins: [
            {
              // The wasm module is embedded into the main app earlier in the build.
              // Exclude it here, otherwise rollup still emits it as a static asset
              // by default.
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
        },
      },
      plugins: [
        guardWasmPlugin(),
        reactPlugin(mode),
        transformPlugin(),
        generateAppHashFile(outputDirectory, ENTRY_FILE_NAME),
        wasm(),
      ],
      assetsInclude: ['**/shared/libs/ironrdp/**/*.wasm'],
      define: {
        'process.env': { NODE_ENV: process.env.NODE_ENV },
      },
    };

    if (process.env.VITE_ANALYZE_BUNDLE) {
      config.plugins.push(visualizer());
    }

    if (mode === 'production') {
      config.base = '/web';

      if (!process.env.VITE_DISABLE_COMPRESSION) {
        config.plugins.push(
          compression({
            algorithms: ['brotliCompress'],
            deleteOriginalAssets: true,
            include: /\.(js|svg|wasm)$/,
            threshold: 1024 * 10, // 10KB
            logLevel: 'silent',
          })
        );
      }
    } else {
      config.plugins.push(htmlPlugin(target));

      // siteName matches everything between the slashes (regex format
      // assumes slashes are escaped, e.g. `\/v1\/webapi\/sites\/:site\/connect`).
      const siteName = '([^\\/]+)';

      // All teleport websocket endpoints live under
      // `/v{N}/webapi/(sites/<site>/{connect, desktops/<d>/connect, kube/exec,
      // db/exec, (desktopplayback|sessionrecording|ttyplayback)/...} |
      // command/<cmd>/execute)`. One alternation covers the lot.
      const wsPath =
        `^\\/v[0-9]+\\/webapi\\/(` +
        [
          `sites\\/${siteName}\\/connect`,
          `sites\\/${siteName}\\/desktops\\/${siteName}\\/connect`,
          `sites\\/${siteName}\\/(kube|db)\\/exec`,
          `sites\\/${siteName}\\/(desktopplayback|sessionrecording|ttyplayback)\\/.+`,
          `command\\/.+\\/execute`,
        ].join('|') +
        `)`;

      config.server.proxy = Object.fromEntries([
        wsRoute(target, wsPath),
        httpRoute(target, '/web/config.js'),
        httpRoute(target, '^\\/v[0-9]+'),
        httpRoute(target, '/enterprise'),
      ]);

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

function resolveAllowedHosts(target: string) {
  const allowedHosts = new Set<string>();

  if (process.env.VITE_HOST) {
    const { hostname } = new URL(`https://${process.env.VITE_HOST}`);

    allowedHosts.add(hostname);
  }

  if (target !== DEFAULT_PROXY_TARGET) {
    const { hostname } = new URL(`https://${target}`);

    allowedHosts.add(hostname);
  }

  return Array.from(allowedHosts);
}

function resolveTargetURL(url: string) {
  if (!url) {
    return;
  }

  const target = url.startsWith('https') ? url : `https://${url}`;

  const parsed = new URL(target);

  return parsed.host;
}

function wsRoute(target: string, path: string): [string, ProxyOptions] {
  return [
    path,
    {
      target: `wss://${target}`,
      secure: false,
      ws: true,
      changeOrigin: true,
      rewriteWsOrigin: true,
    },
  ];
}

function httpRoute(target: string, path: string): [string, ProxyOptions] {
  return [
    path,
    {
      target: `https://${target}`,
      secure: false,
      changeOrigin: true,
    },
  ];
}
