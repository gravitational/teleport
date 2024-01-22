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

import path, { dirname, join, resolve } from 'path';
import { Plugin, UserConfig } from 'vite';
import type { StorybookConfig } from '@storybook/react-vite';
import fs from 'fs';

const enterpriseTeleportExists = fs.existsSync(
  path.join(__dirname, '/../../e/web')
);

function getAbsolutePath(value: string): any {
  return dirname(require.resolve(join(value, 'package.json')));
}

function createStoriesPaths() {
  const stories = ['../packages/**/*.story.@(ts|tsx)'];

  // include enterprise stories if available (**/* pattern ignores dot dir names)
  if (enterpriseTeleportExists) {
    stories.unshift('../../e/web/**/*.story.@(ts|tsx)');
  }

  return stories;
}

const rootDirectory = path.resolve(__dirname, '..', '..');
const webDirectory = path.resolve(rootDirectory, 'web');

const config: StorybookConfig = {
  stories: createStoriesPaths(),
  addons: [getAbsolutePath('@storybook/addon-toolbars')],
  framework: {
    name: getAbsolutePath('@storybook/react-vite'),
    options: {
      builder: {
        viteConfigPath: resolve(
          __dirname,
          '../../web/packages/teleport/vite.config.mts'
        ),
      },
    },
  },
  staticDirs: ['public'],
  async viteFinal(config: UserConfig) {
    config.plugins = config.plugins.filter(
      (plugin: Plugin) =>
        plugin.name !== 'teleport-html-plugin' &&
        plugin.name !== 'teleport-transform-html-plugin'
    );

    return config;
  },
};

export default config;
