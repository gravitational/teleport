/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

const path = require('path');
const fs = require('fs');
const configFactory = require('@gravitational/build/webpack/webpack.base');

// include open source stories
const stories = ['../packages/**/*.story.@(js|jsx|ts|tsx)'];

const tsconfigPath = path.join(__dirname, '../../tsconfig.json');

// include enterprise stories if available (**/* pattern ignores dot dir names)
if (fs.existsSync(path.join(__dirname, '/../../e/'))) {
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
