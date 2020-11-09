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

const fs = require('fs');
const path = require('path');
const HtmlWebPackPlugin = require('html-webpack-plugin');

const appDirectory = fs.realpathSync(process.cwd());
const resolveApp = relativePath => path.resolve(appDirectory, relativePath);

module.exports = function createConfig() {
  return {
    optimization: {
      splitChunks: {
        cacheGroups: {
          vendors: {
            chunks: 'all',
            name: 'vendor',
            test: /([\\/]node_modules[\\/])/,
            priority: -10,
          },
        },
      },
    },

    entry: {
      app: ['./src/boot'],
    },

    output: {
      // used by loaders to generate various URLs within CSS, JS based off publicPath
      publicPath: '/web/app/',

      path: resolveApp('dist/app'),

      /*
       * format of the output file names. [name] stands for 'entry' keys
       * defined in the 'entry' section
       **/
      filename: '[name].[hash].js',

      // chunk file name format
      chunkFilename: '[name].[chunkhash].js',
    },

    resolve: {
      // some vendor libraries expect below globals to be defined
      alias: {
        jQuery: 'jquery',
        teleport: path.join(__dirname, '/../../teleport/src'),
        'e-teleport': path.join(__dirname, '/../../webapps.e/teleport/src'),
        'e-gravity': path.join(__dirname, '/../../webapps.e/gravity/src'),
        design: path.join(__dirname, '/../../design/src'),
        shared: path.join(__dirname, '/../../shared'),
        gravity: path.join(__dirname, '/../../gravity/src'),
      },

      /*
       * root path to resolve js our modules, enables us to use absolute path.
       * For ex: require('./../../../config') can be replaced with require('app/config')
       **/
      modules: ['node_modules', 'src'],
      extensions: ['.ts', '.tsx', '.js', '.jsx'],
    },

    noParse: function(content) {
      return /xterm.js$/.test(content);
    },

    rules: {
      fonts: {
        test: /fonts\/(.)+\.(woff|woff2|ttf|eot|svg)/,
        loader: 'url-loader',
        options: {
          limit: 102400, // 100kb
          name: '/assets/fonts/[name].[ext]',
        },
      },

      svg: {
        test: /\.svg$/,
        loader: 'svg-url-loader',
        options: {
          noquotes: true,
        },
        exclude: /node_modules/,
      },

      css() {
        return {
          test: /\.(css)$/,
          use: ['style-loader', 'css-loader'],
        };
      },

      images: {
        test: /\.(png|jpg|gif|ico)$/,
        loader: 'url-loader',
        options: {
          limit: 10000,
          name: '/assets/img/img-[hash:6].[ext]',
        },
      },
      jsx(args) {
        args = args || {};
        var emitWarning = false;
        if (args.withHot) {
          emitWarning = true;
        }

        return {
          test: /\.(ts|tsx|js|jsx)$/,
          exclude: /(node_modules)|(assets)/,
          use: [
            {
              loader: 'babel-loader',
            },
            {
              loader: 'eslint-loader',
              options: {
                emitWarning,
              },
            },
          ],
        };
      },
    },

    plugins: {
      // builds index html page, the main entry point for application
      createIndexHtml(options) {
        return createHtmlPluginInstance({
          filename: '../index.html',
          title: '',
          inject: true,
          template: path.join(__dirname, '/../index.ejs'),
          ...options,
        });
      },
    },
  };
};

function createHtmlPluginInstance(cfg) {
  cfg.inject = true;
  return new HtmlWebPackPlugin(cfg);
}
