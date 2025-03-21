/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import fs from 'node:fs';
import path from 'node:path';

import type { StorybookConfig } from '@storybook/react-vite';

const enterpriseTeleportExists = fs.existsSync(
  path.join(__dirname, '/../../e/web')
);

function createStoriesPaths() {
  const stories = ['../packages/**/*.story.@(ts|tsx|js|jsx)'];

  // include enterprise stories if available (**/* pattern ignores dot dir names)
  if (enterpriseTeleportExists) {
    stories.unshift('../../e/web/**/*.story.@(ts|tsx|js|jsx)');
  }

  return stories;
}

const config: StorybookConfig = {
  stories: createStoriesPaths(),
  framework: {
    name: '@storybook/react-vite',
    options: { builder: { viteConfigPath: 'web/.storybook/vite.config.mts' } },
  },
  staticDirs: ['public'],
  addons: [
    '@storybook/addon-toolbars',
    '@storybook/addon-controls',
    '@storybook/addon-actions',
  ],
};

export default config;
