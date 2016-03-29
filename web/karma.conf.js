/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

var path = require('path');
var webpack = require('webpack');

// we want to pass this information to the client to disable/enable debug information if needed.
var clientArgs = [];
if(process.env.TELEPORT_NO_DEBUG === 1){
  clientArgs['TELEPORT_NO_DEBUG'];
}

module.exports = function (config) {
  config.set({
    browsers: [],
    frameworks: [ 'mocha' ],
    reporters: [ 'spec' ],
    client: {
      args : [process.env.TELEPORT_NO_DEBUG]
    },
    files: [
      'node_modules/phantomjs-polyfill/bind-polyfill.js',
      'src/assets/js/jquery-2.1.1.js',
      'src/assets/js/bootstrap-3.3.6.js',
      'src/assets/js/term-0.0.7.js',
      'src/assets/js/jquery-validate-1.14.0.js',
      'src/assets/js/underscore-1.8.3.js',
      'tests.webpack.js'
    ],

    preprocessors: {
      'tests.webpack.js': [ 'webpack', 'sourcemap' ]
    },

    webpack: {
      devtool: 'inline-source-map',
      externals: ['jQuery', 'Terminal', '_' ],
      resolve: {
        root: [ path.join(__dirname, 'src') ]
      },
      module: {
        loaders: [
          { test: /\.(js|jsx)$/, exclude: /node_modules/, loader: 'babel' }
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
