import path from 'node:path';
import { defineConfig } from 'vite';
import tsconfigPaths from 'vite-tsconfig-paths';
const outputDirectory = path.resolve(__dirname, 'build');

function tsconfigPathsPlugin() {
  return tsconfigPaths({
    root: path.resolve(import.meta.dirname, '../..'),
  });
}

export default defineConfig(() => ({
  plugins: [tsconfigPathsPlugin()],
  build: {
    lib: {
      name: 'event-fixtures',
      entry: path.resolve(__dirname, '../../web/packages/teleport/src/Audit/fixtures/index.ts'),
      fileName: 'event-fixtures',
      formats: ['es' as const],
    },
    outDir: path.resolve(outputDirectory, 'events'),
  },
}));
