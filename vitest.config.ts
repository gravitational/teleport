/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import path from 'path';
import react from '@vitejs/plugin-react-swc';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [
    react({
      plugins: [['@swc/plugin-styled-components', {}]],
    }),
  ],
  resolve: {
    alias: {
      // Match Jest's moduleNameMapper
      shared: path.resolve(__dirname, './web/packages/shared'),
      design: path.resolve(__dirname, './web/packages/design/src'),
      teleport: path.resolve(__dirname, './web/packages/teleport/src'),
      teleterm: path.resolve(__dirname, './web/packages/teleterm/src'),
      'e-teleport': path.resolve(__dirname, './e/web/teleport/src'),
      'gen-proto-js': path.resolve(__dirname, './gen/proto/js'),
      'gen-proto-ts': path.resolve(__dirname, './gen/proto/ts'),
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    include: [
      'web/packages/build/**/*.test.{ts,tsx}',
      'web/packages/design/**/*.test.{ts,tsx}',
      'web/packages/shared/**/*.test.{ts,tsx}',
      'web/packages/teleport/**/*.test.{ts,tsx}',
      'web/packages/teleterm/**/*.test.{ts,tsx}',
      'e/web/teleport/**/*.test.{ts,tsx}',
    ],
    setupFiles: [
      path.resolve(__dirname, './web/packages/build/vitest/setupTests.ts'),
      path.resolve(__dirname, './web/packages/build/vitest/customMatchers.ts'),
    ],
    env: {
      TZ: 'UTC',
    },
  },
});
