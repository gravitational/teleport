import path from 'node:path';

import { defineConfig, type Plugin } from 'vite';

import { tsconfigPathsPlugin } from './tsconfigPaths.mjs';

// stubTeleportConfigPlugin intercepts the 'teleport/config' import and
// returns a minimal stub to avoid bundling the large config module, which
// also contains browser code.
function stubTeleportConfigPlugin(): Plugin {
  const virtualModuleId = '\0teleport/config';
  return {
    name: 'stub-teleport-config',
    // Execute the plugin before vite-tsconfig-paths (which also uses enforce:
    // 'pre').
    enforce: 'pre',
    resolveId(id) {
      if (id === 'teleport/config') return virtualModuleId;
    },
    load(id) {
      if (id === virtualModuleId) {
        return 'const cfg = { getIntegrationEnrollRoute: () => "" }; export default cfg;';
      }
    },
  };
}

const rootDirectory = path.resolve(
  __dirname,
  '../../teleport/src/Discover/SelectResource/gen-guided-resources'
);
const outputDirectory = path.resolve(
  __dirname,
  '../../teleport/src/Discover/SelectResource/gen-guided-resources/dist'
);

export default defineConfig(() => ({
  plugins: [stubTeleportConfigPlugin(), tsconfigPathsPlugin()],
  build: {
    outDir: outputDirectory,
    minify: false,
    cssMinify: false,
    reportCompressedSize: false,
    lib: {
      name: 'gen-guided-resources',
      fileName: 'gen-guided-resources',
      entry: path.resolve(rootDirectory, 'index.ts'),
      formats: ['cjs' as const],
    },
    rollupOptions: {
      external: ['node:fs'],
    },
  },
}));
