#!/usr/bin/env node
const path = require('path');

const spawn = require('cross-spawn');

if (!process.argv.some(arg => arg.startsWith('--config'))) {
  process.argv.push(
    '--config',
    path.join(__dirname, '../webpack/webpack.prod.config.js')
  );
}

const args = process.argv.slice(2);

spawn.sync('webpack', args, { stdio: 'inherit' });
