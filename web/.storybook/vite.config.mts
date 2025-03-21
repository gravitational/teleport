import { defineConfig } from 'vite';

import { reactPlugin } from '../packages/build/vite/react';
import { tsconfigPathsPlugin } from '../packages/build/vite/tsconfigPaths';

export default defineConfig(({ mode }) => ({
  plugins: [tsconfigPathsPlugin(), reactPlugin(mode)],
}));
