const webpack = require('webpack');
const path = require('path');
const createBaseDefaults = require('@gravitational/build/webpack/webpack.base');
const defaultCfg = require('@gravitational/build/webpack/webpack.dev.config');

module.exports = {
  ...defaultCfg,
  plugins: [
    ...defaultCfg.plugins,
    createBaseDefaults().plugins.createIndexHtml({
      favicon: path.join(__dirname, '/src/favicon.ico'),
    }),
  ],
};
