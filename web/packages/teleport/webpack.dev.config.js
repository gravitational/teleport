const path = require('path');

const configFactory = require('@gravitational/build/webpack/webpack.base');
const defaultDevConfig = require('@gravitational/build/webpack/webpack.dev.config');

/**
 * @type { import("webpack").webpack.Configuration }
 */
module.exports = {
  ...defaultDevConfig,
  plugins: [
    ...defaultDevConfig.plugins,
    configFactory.plugins.indexHtml({
      favicon: path.join(__dirname, '/src/favicon-light.png'),
    }),
  ],
};
