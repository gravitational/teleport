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

const webpack = require('webpack');
const createConfig = require('./webpack.base');

process.env.BABEL_ENV = 'development';
process.env.NODE_ENV = 'development';

const baseCfg = createConfig();

var cfg = {
  entry: baseCfg.entry,
  output: {
    ...baseCfg.output,
    filename: '[name].js',
    chunkFilename: '[name].js',
  },
  resolve: {
    ...baseCfg.resolve,
    alias: {
      ...baseCfg.resolve.alias,
      'react-dom': '@hot-loader/react-dom',
    },
  },

  devtool: false,

  mode: 'development',

  optimization: baseCfg.optimization,

  module: {
    noParse: baseCfg.noParse,
    strictExportPresence: true,
    rules: [
      baseCfg.rules.fonts,
      baseCfg.rules.svg,
      baseCfg.rules.images,
      baseCfg.rules.jsx({ withHot: true }),
      baseCfg.rules.css(),
    ],
  },

  plugins: [
    new webpack.HotModuleReplacementPlugin(),
    baseCfg.plugins.createIndexHtml(),
  ],
};

module.exports = cfg;
