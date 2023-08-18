const path = require('path');
const { spawn } = require('child_process');

const { CleanWebpackPlugin } = require('clean-webpack-plugin');
const CopyPlugin = require('copy-webpack-plugin');
const resolvepath = require('@gravitational/build/webpack/resolvepath');
const configFactory = require('@gravitational/build/webpack/webpack.base');

function onFirstBuildDonePlugin(env) {
  let isInitialBuild = true;
  return {
    apply: compiler => {
      compiler.hooks.done.tap('OnFirstBuildDonePlugin', (/*compilation*/) => {
        if (!isInitialBuild) {
          return;
        }
        isInitialBuild = false;

        const child = spawn('yarn', ['start-electron', '--inspect'], {
          shell: true,
          env,
          stdio: 'inherit',
          detached: true, // detaching the process will allow restarting electron without terminating the dev server
        });

        child.unref();
      });
    },
  };
}

const cfg = {
  entry: {
    main: './src/main.ts',
    preload: './src/preload.ts',
    sharedProcess: './src/sharedProcess/sharedProcess.ts',
  },

  output: {
    path: resolvepath('build/app/dist/main'),
    filename: '[name].js',
  },

  resolve: {
    ...configFactory.createDefaultConfig().resolve,
    alias: {
      ...configFactory.createDefaultConfig().resolve.alias,
      teleterm: path.join(__dirname, './src'),
    },
  },

  devtool: false,

  target: 'electron-main',

  optimization: {
    minimize: false,
  },

  module: {
    strictExportPresence: true,
    rules: [configFactory.rules.jsx()],
  },

  externals: {
    'node-pty': 'commonjs2 node-pty',
  },

  plugins: [
    new CleanWebpackPlugin(),
    // Instead of adding agentCleanupDaemon.js as a separate entry, copy it to the dist dir as is.
    // We want the daemon in prod to be as close as possible to the daemon in tests.
    // See agentCleanupDaemon.js for the rationale.
    new CopyPlugin({
      patterns: [
        {
          from: './src/agentCleanupDaemon/agentCleanupDaemon.js',
          to: 'agentCleanupDaemon.js',
        },
      ],
    }),
  ],

  /**
   * Disables webpack processing of __dirname and __filename.
   * If you run the bundle in node.js it falls back to these values of node.js.
   * https://github.com/webpack/webpack/issues/2010
   */
  node: {
    __dirname: false,
    __filename: false,
  },
};

module.exports = (env, argv) => {
  if (argv.mode === 'development') {
    process.env.BABEL_ENV = 'development';
    process.env.NODE_ENV = 'development';
    cfg.mode = 'development';
    cfg.plugins.push(onFirstBuildDonePlugin(process.env));
  }

  if (argv.mode === 'production') {
    process.env.BABEL_ENV = 'production';
    process.env.NODE_ENV = 'production';
    cfg.mode = 'production';
  }

  return cfg;
};
