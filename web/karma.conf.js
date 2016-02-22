var path = require('path');
var webpack = require('webpack');

module.exports = function (config) {
  // Browsers to run on BrowserStack
  var customLaunchers = {
    BS_Chrome: {
      base: 'BrowserStack',
      os: 'Windows',
      os_version: '8.1',
      browser: 'chrome',
      browser_version: '39.0',
    }
  };

  config.set({
    customLaunchers: customLaunchers,
    browsers: [ 'Chrome'],
    frameworks: [ 'mocha' ],
    reporters: [ 'mocha' ],
    files: [
      'node_modules/phantomjs-polyfill/bind-polyfill.js',
      'src/assets/js/jquery-2.1.1.js',
      'src/assets/js/bootstrap.min.js',
      'src/assets/js/plugins/metisMenu/jquery.metisMenu.js',
      'src/assets/js/plugins/slimscroll/jquery.slimscroll.min.js',
      'src/assets/js/term.js',
      'src/assets/js/jquery-validate-1.14.0.js',
      'tests.webpack.js'
    ],

    preprocessors: {
      'tests.webpack.js': [ 'webpack', 'sourcemap' ]
    },

    webpack: {
      devtool: 'inline-source-map',
      externals: ['jQuery', 'Terminal', 'sinos' ],
      resolve: {
        root: [ path.join(__dirname, 'src') ]
      },
      module: {
        loaders: [
          { test: /\.js$/, exclude: /node_modules/, loader: 'babel' },
          { test: /\.jsx$/, exclude: /node_modules/, loader: 'babel' },
        ]
      },
      plugins: [
        new webpack.DefinePlugin({
          'process.env.NODE_ENV': JSON.stringify('test')
        })
      ]
    },

    webpackServer: {
      noInfo: true
    }
  });
};
