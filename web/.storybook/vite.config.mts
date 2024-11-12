import { defineConfig } from 'vite';
import { tsconfigPathsPlugin } from '@gravitational/build/vite/tsconfigPaths.mjs';
import { reactPlugin } from '@gravitational/build/vite/react.mjs';

export default defineConfig(({ mode }) => ({
  plugins: [tsconfigPathsPlugin(), reactPlugin(mode)],
}));
