import path from 'node:path';

import { defineConfig } from 'vite';

import { tsconfigPathsPlugin } from './tsconfigPaths.mjs';

const rootDirectory = path.resolve(
  __dirname,
  '../../teleport/src/services/audit/gen-event-reference'
);
const outputDirectory = path.resolve(
  __dirname,
  '../../teleport/src/services/audit/gen-event-reference/dist'
);

export default defineConfig(() => ({
  plugins: [tsconfigPathsPlugin()],
  build: {
    outDir: outputDirectory,
    minify: false,
    cssMinify: false,
    reportCompressedSize: false,
    lib: {
      name: 'event-fixtures',
      fileName: 'event-fixtures',
      entry: path.resolve(rootDirectory, 'index.ts'),
      formats: ['cjs' as const],
    },
    rollupOptions: {
      external: ['node:fs'],
    },
  },
}));
