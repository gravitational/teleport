import { defineConfig } from 'vite';

import { reactPlugin } from '@gravitational/build/vite/react.mjs';
import { tsconfigPathsPlugin } from '@gravitational/build/vite/tsconfigPaths.mjs';

export default defineConfig(({ mode }) => ({
  plugins: [tsconfigPathsPlugin(), reactPlugin(mode)],
}));
