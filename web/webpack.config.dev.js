var webpack = require('webpack');
var webpackConfig = require('./webpack.config.js');

webpackConfig.cache = true;
webpackConfig.devtool = 'inline-source-map';
webpackConfig.plugins = [
  new webpack.optimize.OccurenceOrderPlugin(),
  new webpack.optimize.CommonsChunkPlugin({ names: ['vendor'] })]

module.exports = webpackConfig;
