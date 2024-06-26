const path = require('path');

const { CleanWebpackPlugin } = require('clean-webpack-plugin');
const configFactory = require('@gravitational/build/webpack/webpack.base');
const defaultProdConfig = require('@gravitational/build/webpack/webpack.prod.config');

/**
 * @type { import("webpack").webpack.Configuration }
 */
module.exports = {
  ...defaultProdConfig,
  optimization: {
    ...defaultProdConfig.optimization,
    moduleIds: 'deterministic',
  },
  plugins: [
    ...defaultProdConfig.plugins,
    new CleanWebpackPlugin(),
    configFactory.plugins.indexHtml({
      favicon: path.join(__dirname, '/src/favicon-light.png'),
    }),
  ],
};
