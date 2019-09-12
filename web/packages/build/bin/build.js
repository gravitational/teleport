#!/usr/bin/env node
const fs = require('fs');
const path = require('path');

const defaultConfig = process.argv.some(arg => arg.startsWith('--dev'))
  ? 'webpack.dev.config.js'
  : 'webpack.prod.config.js';

if (!process.argv.some(arg => arg.startsWith('--config'))) {
  let webpackConfig = path.join(process.cwd(), 'webpack.config.js');
  if (!fs.existsSync(webpackConfig)) {
    webpackConfig = path.join(__dirname, '../webpack', defaultConfig);
  }

  process.argv.push('--config', webpackConfig);
}

require('webpack-cli');
