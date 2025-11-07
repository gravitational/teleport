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
import { UserConfig } from 'vitest/config';

export const baseConfig: UserConfig = {
  plugins: [
    react({
      plugins: [['@swc/plugin-styled-components', {}]],
    }),
  ],
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: [
      path.resolve(__dirname, './setupTests.ts'),
      path.resolve(__dirname, '../jest/customMatchers.ts'),
    ],
    env: {
      TZ: 'UTC',
    },
  },
};
