import path from 'node:path';

import { defineConfig, type Plugin } from 'vite';

// stubTeleportConfigPlugin intercepts the 'teleport/config' import and returns
// a minimal stub to avoid bundling the large config module, which breaks the
// generator since it relies on browser APIs.
function stubTeleportConfigPlugin(): Plugin {
  const virtualModuleId = '\0teleport/config';
  return {
    name: 'stub-teleport-config',
    // Execute the plugin before vite-tsconfig-paths (which also uses enforce:
    // 'pre').
    enforce: 'pre',
    // In resolveId, the id is the module identifier as specified in an import
    // statement. This hook overrides the default behavior of resolving id to
    // an absolute file path. Instead, return a virtual module ID, which by
    // convention in the bundler (Rollup), begins with "\0". This way, the
    // load hook receives the virtual module ID, rather than a file path, so
    // it can match id against the expected string and return a stub.
    resolveId(id) {
      if (id === 'teleport/config') return virtualModuleId;
    },
    load(id) {
      if (id.includes('teleport/config')) {
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
  resolve: {
    tsconfigPaths: true,
  },
  plugins: [stubTeleportConfigPlugin()],
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
    rolldownOptions: {
      external: ['node:fs'],
    },
  },
}));
