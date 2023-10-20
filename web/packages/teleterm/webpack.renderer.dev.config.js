const { spawn } = require('child_process');
const path = require('path');

const defaultCfg = require('@gravitational/build/webpack/webpack.dev.config');
const configFactory = require('@gravitational/build/webpack/webpack.base');

const { extend } = require('./webpack.renderer.extend');
const devCfg = extend(defaultCfg);

devCfg.devServer = {
  hot: true,
  static: {
    publicPath: '/',
    directory: path.join(__dirname, 'build/app/dist/renderer'),
    serveIndex: false,
  },
  allowedHosts: 'all',
  server: {
    type: 'https',
  },
  onBeforeSetupMiddleware() {
    spawn('yarn', ['start-main'], {
      shell: true,
      env: process.env,
      stdio: 'inherit',
    }).on('error', spawnError => console.error(spawnError));
  },
};

devCfg.output.publicPath = '';
devCfg.plugins.push(
  configFactory.plugins.tsChecker(),
  configFactory.plugins.reactRefresh()
);

module.exports = devCfg;
