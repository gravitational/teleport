/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
