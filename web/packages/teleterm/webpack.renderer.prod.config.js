const { CleanWebpackPlugin } = require('clean-webpack-plugin');

const defaultCfg = require('@gravitational/build/webpack/webpack.prod.config');

const { extend } = require('./webpack.renderer.extend');

const prodCfg = extend(defaultCfg);

prodCfg.plugins.unshift(new CleanWebpackPlugin());

module.exports = prodCfg;
