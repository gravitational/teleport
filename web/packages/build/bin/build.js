#!/usr/bin/env node
const path = require('path');

if (!process.argv.some(arg => arg.startsWith('--config'))) {
  process.argv.push(
    '--config',
    path.join(__dirname, '../webpack/webpack.config.js')
  );
}

require('webpack-cli');
