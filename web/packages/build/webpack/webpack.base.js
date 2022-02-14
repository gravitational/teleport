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
const HtmlWebPackPlugin = require('html-webpack-plugin');
const ReactRefreshPlugin = require('@pmmmwh/react-refresh-webpack-plugin');

const resolvePath = require('./resolvepath');

const configFactory = {
  createDefaultConfig,
  plugins: {
    reactRefresh(options) {
      return new ReactRefreshPlugin(options);
    },
    indexHtml(options) {
      return new HtmlWebPackPlugin({
        filename: '../index.html',
        title: '',
        inject: true,
        template: path.join(__dirname, '/../index.ejs'),
        ...options,
      });
    },
  },
  rules: {
    fonts() {
      return {
        test: /fonts\/(.)+\.(woff|woff2|ttf|eot|svg)/,
        loader: 'url-loader',
        options: {
          limit: 102400, // 100kb
          name: '/assets/fonts/[name].[ext]',
        },
      };
    },
    svg() {
      return {
        test: /\.svg$/,
        loader: 'svg-url-loader',
        exclude: /node_modules/,
      };
    },
    css() {
      return {
        test: /\.(css)$/,
        use: ['style-loader', 'css-loader'],
      };
    },
    images() {
      return {
        test: /\.(png|jpg|gif|ico)$/,
        loader: 'url-loader',
        options: {
          limit: 10000,
          name: '/assets/img/img-[hash:6].[ext]',
        },
      };
    },
    jsx() {
      return {
        test: /\.(ts|tsx|js|jsx)$/,
        exclude: /(node_modules)|(assets)/,
        use: [
          {
            loader: 'babel-loader',
          },
        ],
      };
    },
  },
};

/** @return {import('webpack').webpack.Configuration} */
function createDefaultConfig() {
  return {
    optimization: {
      splitChunks: {
        cacheGroups: {
          // Vendor chunk creates a chunk file that contains files coming from import statements
          // from node_modules. The 'initial' flag directs this group to add modules to this chunk
          //  that were imported inside only from sync chunks.
          defaultVendors: {
            chunks: 'initial',
            name: 'vendor',
            test: /([\\/]node_modules[\\/])/,
            // Priority states that if a module falls under many cacheGroups, then
            // the module will be part of a chunk with a higher priority.
          },
          // Common chunk creates a chunk file that contains modules that were shared between
          // at least 2 (or more) async chunks. The 'async' flag directs this group to add modules
          // to this chunk that were specifically imported inside async chunks (dynamic imports).
          common: {
            chunks: 'async',
            minChunks: 2,
            test: /([\\/]node_modules[\\/])/,
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

      path: resolvePath('dist/app'),

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
        teleterm: path.join(__dirname, '/../../teleterm/src'),
        teleport: path.join(__dirname, '/../../teleport/src'),
        'e-teleport': path.join(__dirname, '/../../webapps.e/teleport/src'),
        design: path.join(__dirname, '/../../design/src'),
        shared: path.join(__dirname, '/../../shared'),
      },

      /*
       * root path to resolve js our modules, enables us to use absolute path.
       * For ex: require('./../../../config') can be replaced with require('app/config')
       **/
      modules: ['node_modules', 'src'],
      extensions: ['.ts', '.tsx', '.js', '.jsx'],
    },
  };
}


module.exports = configFactory;