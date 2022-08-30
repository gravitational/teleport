#!/usr/bin/env node

/* eslint-disable no-console */

process.on('unhandledRejection', err => {
  throw err;
});

const path = require('path');

const spawn = require('cross-spawn');

if (!process.argv.some(arg => arg.startsWith('--config'))) {
  const defaultWebpackConfig = path.join(
    __dirname,
    '../webpack/webpack.dev.config.js'
  );
  process.argv.push(`--config=${defaultWebpackConfig}`);
}

const args = process.argv.slice(2);
const devServerPath = path.join(__dirname, '../devserver');
const nodeArgs = [devServerPath].concat(args);

const result = spawn.sync('node', nodeArgs, { stdio: 'inherit' });
if (result.signal) {
  if (result.signal === 'SIGKILL') {
    console.log(
      'The build failed because the process exited too early. ' +
        'This probably means the system ran out of memory or someone called ' +
        '`kill -9` on the process.'
    );
  } else if (result.signal === 'SIGTERM') {
    console.log(
      'The build failed because the process exited too early. ' +
        'Someone might have called `kill` or `killall`, or the system could ' +
        'be shutting down.'
    );
  }
  process.exit(1);
}
process.exit(result.status);
