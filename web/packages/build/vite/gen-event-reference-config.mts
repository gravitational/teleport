import path from 'node:path';

import { defineConfig } from 'vite';

const rootDirectory = path.resolve(
  __dirname,
  '../../teleport/src/services/audit/gen-event-reference'
);
const outputDirectory = path.resolve(
  __dirname,
  '../../teleport/src/services/audit/gen-event-reference/dist'
);

export default defineConfig(() => ({
  resolve: {
    tsconfigPaths: true,
  },
  build: {
    outDir: outputDirectory,
    minify: false,
    cssMinify: false,
    reportCompressedSize: false,
    lib: {
      name: 'gen-event-reference',
      fileName: 'gen-event-reference',
      entry: path.resolve(rootDirectory, 'index.ts'),
      formats: ['cjs' as const],
    },
    rolldownOptions: {
      external: ['node:fs'],
    },
  },
}));
