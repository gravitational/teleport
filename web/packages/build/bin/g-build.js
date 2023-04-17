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

const result = spawn.sync('webpack', args, { stdio: 'inherit' });
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
