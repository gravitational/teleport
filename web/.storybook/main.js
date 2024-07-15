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

const path = require('path');
const fs = require('fs');
const configFactory = require('@gravitational/build/webpack/webpack.base');

// include open source stories
const stories = ['../packages/**/*.story.@(js|jsx|ts|tsx)'];

const tsconfigPath = path.join(__dirname, '../../tsconfig.json');

const enterpriseTeleportExists = fs.existsSync(
  path.join(__dirname, '/../../e/web')
);

// include enterprise stories if available (**/* pattern ignores dot dir names)
if (enterpriseTeleportExists) {
  stories.unshift('../../e/web/**/*.story.@(js|jsx|ts|tsx)');
}

module.exports = {
  core: {
    builder: 'webpack5',
  },
  reactOptions: {
    fastRefresh: true,
  },
  typescript: {
    reactDocgen: false,
  },
  addons: ['@storybook/addon-toolbars'],
  stories,
  webpackFinal: async (storybookConfig, { configType }) => {
    // configType has a value of 'DEVELOPMENT' or 'PRODUCTION'
    // You can change the configuration based on that.
    // 'PRODUCTION' is used when building the static version of storybook.
    storybookConfig.devtool = false;
    storybookConfig.resolve = {
      ...storybookConfig.resolve,
      ...configFactory.createDefaultConfig().resolve,
    };

    // Access Graph requires a separate repo to be cloned. At the moment, only the Vite config is
    // configured to resolve access-graph. However, Storybook uses Webpack and since our usual
    // Webpack config doesn't need to know about access-graph, we manually to manually configure
    // Storybook's Webpack here to resolve access-graph to the special mock.
    //
    // See https://github.com/gravitational/teleport.e/issues/2675.
    storybookConfig.resolve.alias['access-graph'] = path.join(
      __dirname,
      'mocks',
      'AccessGraph.tsx'
    );

    if (!enterpriseTeleportExists) {
      delete storybookConfig.resolve.alias['e-teleport'];
    }

    storybookConfig.optimization = {
      splitChunks: {
        cacheGroups: {
          stories: {
            maxSize: 500000, // 500kb
            chunks: 'all',
            name: 'stories',
            test: /packages/,
          },
        },
      },
    };

    storybookConfig.module.rules.push({
      resourceQuery: /raw/,
      type: 'asset/source',
    });

    storybookConfig.module.rules.push({
      test: /\.(ts|tsx)$/,
      use: [
        {
          loader: require.resolve('babel-loader'),
        },
        {
          loader: require.resolve('ts-loader'),
          options: {
            onlyCompileBundledFiles: true,
            configFile: tsconfigPath,
            transpileOnly: configType === 'DEVELOPMENT',
            compilerOptions: {
              jsx: 'preserve',
            },
          },
        },
      ],
    });

    return storybookConfig;
  },
};
