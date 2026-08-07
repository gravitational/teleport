import { defineConfig } from 'vite';

import { reactPlugin } from '@gravitational/build/vite/react.mjs';

export default defineConfig(({ mode }) => ({
  resolve: {
    tsconfigPaths: true,
  },
  plugins: [reactPlugin(mode)],
  assetsInclude: ['**/shared/libs/ironrdp/**/*.wasm'],
}));
