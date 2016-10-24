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

var webpackCfg = require('./webpack/webpack.config.test');

module.exports = function (config) {
  config.set({
    browsers: [],
    frameworks: [ 'mocha' ],
    reporters: [ 'spec' ],
    files: [
      'node_modules/phantomjs-polyfill/bind-polyfill.js',
      'karma.test.files.js'
    ],

    preprocessors: {
      'karma.test.files.js': [ 'webpack' ]
    },

    webpack: webpackCfg,

    webpackServer: {
      noInfo: true
    }
  });
};
