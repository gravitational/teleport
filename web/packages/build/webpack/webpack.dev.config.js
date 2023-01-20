/*
Copyright 2019 Gravitational, Inc.

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

const configFactory = require('./webpack.base');

process.env.BABEL_ENV = 'development';
process.env.NODE_ENV = 'development';

function getCacheConfig() {
  if (process.env.WEBPACK_CACHE_DISABLED === 'yes') {
    return undefined;
  }

  const cache = {
    type: 'filesystem',
  };

  if (process.env.WEBPACK_CACHE_DIRECTORY) {
    cache.cacheDirectory = process.env.WEBPACK_CACHE_DIRECTORY;
  }

  return cache;
}

const plugins = [
  configFactory.plugins.tsChecker(),
  configFactory.plugins.reactRefresh(),
];

if (process.env.WEBPACK_ANALYZE_BUNDLE === 'true') {
  plugins.push(configFactory.plugins.bundleAnalyzer());
}

/**
 * @type { import('webpack').webpack.Configuration }
 */
module.exports = {
  ...configFactory.createDefaultConfig(),
  cache: getCacheConfig(),
  output: {
    ...configFactory.createDefaultConfig().output,
    filename: '[name].js',
    chunkFilename: '[name].js',
  },
  devtool: process.env.WEBPACK_SOURCE_MAP || 'eval-source-map',
  mode: 'development',
  plugins,
  module: {
    strictExportPresence: true,
    rules: [
      configFactory.rules.raw(),
      configFactory.rules.fonts(),
      configFactory.rules.svg(),
      configFactory.rules.images(),
      configFactory.rules.jsx(),
      configFactory.rules.css(),
    ],
  },
};
