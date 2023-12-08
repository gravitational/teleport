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
