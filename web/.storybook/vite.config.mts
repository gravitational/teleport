import { reactPlugin } from '@gravitational/build/vite/react.mjs';
import { tsconfigPathsPlugin } from '@gravitational/build/vite/tsconfigPaths.mjs';
import { defineConfig } from 'vite';

export default defineConfig(({ mode }) => ({
  plugins: [tsconfigPathsPlugin(), reactPlugin(mode)],
}));
