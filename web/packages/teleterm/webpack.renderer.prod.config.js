const defaultCfg = require('@gravitational/build/webpack/webpack.prod.config');
const { extend } = require('./webpack.renderer.extend');

module.exports = extend(defaultCfg);
