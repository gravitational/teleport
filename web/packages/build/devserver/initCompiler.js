/*
Copyright 2019 Gravitational, Inc.

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

const path = require('path');

const webpack = require('webpack');

module.exports = function initCompiler({ webpackConfig }) {
  // These callbacks are used to notify a subscriber when the very
  // first webpack compilation happents
  let pendingCallbacks = [];

  const webpackCompiler = webpack(webpackConfig);

  // ProgressPlugin is used to output the compilation progress to console
  new webpack.ProgressPlugin({
    entries: true,
    modules: false,
    profile: false,
  }).apply(webpackCompiler);

  function isLocalIndexHtmlReady() {
    return webpackCompiler.outputFileSystem.existsSync(
      path.join(process.cwd(), '//dist//index.html')
    );
  }

  function readLocalIndexHtml() {
    return webpackCompiler.outputFileSystem.readFileSync(
      path.join(process.cwd(), '//dist//index.html'),
      'utf8'
    );
  }

  function callWhenReady(cb) {
    pendingCallbacks.push(cb);
  }

  // onReady is called every time when webpack compilation finishes
  function onReady() {
    if (isLocalIndexHtmlReady() && pendingCallbacks.length > 0) {
      const cbs = pendingCallbacks;
      pendingCallbacks = [];
      cbs.forEach(cb => {
        cb();
      });
    }
  }

  // subscribe to 'done' events
  webpackCompiler.hooks.done.tap('GravitationalDevServer', onReady);

  return {
    webpackCompiler,
    isLocalIndexHtmlReady,
    readLocalIndexHtml,
    callWhenReady,
  };
};
