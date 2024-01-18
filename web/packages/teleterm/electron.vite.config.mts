import { bytecodePlugin, defineConfig, externalizeDepsPlugin } from 'electron-vite';
import { resolve } from 'path';
import { cspPlugin } from '../build/vite/csp';
import { getConnectCsp } from './csp';
import { getStyledComponentsConfig } from '../build/vite/styled';
import react from '@vitejs/plugin-react-swc';
import tsconfigPaths from 'vite-tsconfig-paths';
import { existsSync, readFileSync } from 'fs';

const rootDirectory = resolve(__dirname, '../../..');
const outputDirectory = resolve(__dirname, 'build');

const config = defineConfig(env => {
  const tsConfigPathsPlugin = tsconfigPaths({
    projects: [resolve(rootDirectory, 'tsconfig.json')],
  });

  const certsDirectory = resolve(__dirname, 'certs');

  if (!existsSync(certsDirectory)) {
    throw new Error(
      'Could not find SSL certificates. Please follow web/README.md to generate certificates.'
    );
  }

  const keyPath = resolve(certsDirectory, 'server.key');
  const certPath = resolve(certsDirectory, 'server.crt');

  return {
    main: {
      build: {
        outDir: resolve(outputDirectory, 'main'),
        rollupOptions: {
          input: {
            index: resolve(__dirname, 'src/main.ts'),
            sharedProcess: resolve(__dirname, 'src/sharedProcess/sharedProcess.ts'),
            agentCleanupDaemon: resolve(__dirname, 'src/agentCleanupDaemon/agentCleanupDaemon.js'),
          },
          onwarn(warning, warn) {
            if (warning.code === 'EVAL') {
              return;
            }

            warn(warning)
          },
          output: {
            manualChunks(id) {
              if (id.includes('strip-ansi')) {
                return 'strip-ansi';
              }

              if (id.includes('ansi-regex')) {
                return 'ansi-regex';
              }

              if (id.includes('d3-color')) {
                return 'd3-color';
              }
            }
          }
        }
      },
      plugins: [
        externalizeDepsPlugin({ exclude: ['strip-ansi', 'ansi-regex', 'd3-color'] }),
        tsConfigPathsPlugin,
      ],
    },
    preload: {
      build: {
        outDir: resolve(outputDirectory, 'preload'),
        rollupOptions: {
          input: {
            index: resolve(__dirname, 'src/preload.ts')
          },
          onwarn(warning, warn) {
            if (warning.code === 'EVAL') {
              return;
            }

            warn(warning)
          },
          output: {
            manualChunks(id) {
              if (id.includes('strip-ansi')) {
                return 'strip-ansi';
              }

              if (id.includes('ansi-regex')) {
                return 'ansi-regex';
              }

              if (id.includes('d3-color')) {
                return 'd3-color';
              }
            }
          }
        },
      },
      plugins: [
        externalizeDepsPlugin({
          exclude: ['strip-ansi', 'ansi-regex', 'd3-color']
        }),
        tsConfigPathsPlugin,
      ],
    },
    renderer: {
      root: '.',
      build: {
        outDir: resolve(outputDirectory, 'renderer'),
        rollupOptions: {
          input: {
            index: resolve(__dirname, 'index.html')
          },
        },
      },
      server: {
        host: '0.0.0.0',
        port: 8080,
        fs: {
          allow: [rootDirectory, '.'],
        },
        https: {
          key: readFileSync(keyPath),
          cert: readFileSync(certPath),
        },
      },
      plugins: [
        react({
          plugins: [
            ['@swc/plugin-styled-components', getStyledComponentsConfig(env.mode)],
          ],
        }),
        cspPlugin(getConnectCsp(env.mode === 'development')),
        tsConfigPathsPlugin,
      ],
      define: {
        'process.env': {NODE_ENV: process.env.NODE_ENV},
      },
    },
  };
});

export { config as default };
