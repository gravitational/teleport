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
const createConfig = require('@gravitational/build/webpack/webpack.base');

const webpackCfg = createConfig();

module.exports = {
  stories: ['../packages/**/*.story.(js|jsx|tsx)'],

  webpackFinal: async (config, { configType }) => {
    // configType has a value of 'DEVELOPMENT' or 'PRODUCTION'
    // You can change the configuration based on that.
    // 'PRODUCTION' is used when building the static version of storybook.
    config.devtool = false;
    config.resolve = {
      ...config.resolve,
      ...webpackCfg.resolve,
    };

    config.optimization = {
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

    config.module.rules.push({
      test: /\.(ts|tsx)$/,
      loader: require.resolve('babel-loader'),
    });

    return config;
  },
};
