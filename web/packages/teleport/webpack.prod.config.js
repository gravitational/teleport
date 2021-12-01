const path = require('path');
const webpack = require('webpack');
const { CleanWebpackPlugin } = require('clean-webpack-plugin');
const createBaseDefaults = require('@gravitational/build/webpack/webpack.base');
const defaultCfg = require('@gravitational/build/webpack/webpack.prod.config');

module.exports = {
  ...defaultCfg,
  optimization: {
    ...defaultCfg.optimization,
    moduleIds: 'hashed',
  },
  plugins: [
    ...defaultCfg.plugins,
    new CleanWebpackPlugin(),
    createBaseDefaults().plugins.createIndexHtml({
      favicon: path.join(__dirname, '/src/favicon.ico'),
    }),
  ],
};
