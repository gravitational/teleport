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

const { BundleAnalyzerPlugin } = require('webpack-bundle-analyzer');
const HtmlWebPackPlugin = require('html-webpack-plugin');
const ForkTsCheckerWebpackPlugin = require('fork-ts-checker-webpack-plugin');
const ReactRefreshPlugin = require('@pmmmwh/react-refresh-webpack-plugin');

const resolvePath = require('./resolvepath');

const tsconfigPath = path.join(__dirname, '/../../../../tsconfig.json');

const configFactory = {
  createDefaultConfig,
  plugins: {
    reactRefresh(options) {
      return new ReactRefreshPlugin(options);
    },
    tsChecker() {
      return new ForkTsCheckerWebpackPlugin({
        typescript: {
          configFile: tsconfigPath,
          memoryLimit: 4096,
        },
        issue: {
          exclude: [{ file: '**/*.story.tsx' }],
        },
      });
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
    bundleAnalyzer(options) {
      return new BundleAnalyzerPlugin({ analyzerHost: '0.0.0.0', ...options });
    },
  },
  rules: {
    raw() {
      return {
        resourceQuery: /raw/,
        type: 'asset/source',
      };
    },
    fonts() {
      return {
        test: /fonts\/(.)+\.(woff|woff2|ttf|svg)/,
        type: 'asset',
        generator: {
          filename: 'assets/fonts/[name][ext]',
        },
      };
    },
    svg() {
      return {
        test: /\.svg$/,
        type: 'asset/inline',
        exclude: /[\\/]node_modules[\\/]/,
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
        type: 'asset',
        generator: {
          filename: 'assets/img/img-[hash:6][ext]',
        },
        parser: {
          dataUrlCondition: {
            maxSize: 10240, // 10kb
          },
        },
      };
    },
    jsx() {
      return {
        test: /\.(ts|tsx|js|jsx)$/,
        exclude: /[\\/]node_modules[\\/]/,
        use: [
          {
            loader: 'babel-loader',
          },
          {
            loader: 'ts-loader',
            options: {
              onlyCompileBundledFiles: true,
              configFile: tsconfigPath,
            },
          },
        ],
      };
    },
  },
};

/** @return {import('webpack').webpack.Configuration} */
function createDefaultConfig() {
  return {
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
      filename: '[name].[contenthash].js',

      // chunk file name format
      chunkFilename: '[name].[chunkhash].js',
    },

    resolve: {
      // some vendor libraries expect below globals to be defined
      alias: {
        teleterm: path.join(__dirname, '/../../teleterm/src'),
        teleport: path.join(__dirname, '/../../teleport/src'),
        'e-teleport': path.join(__dirname, '/../../../../e/web/teleport/src'),
        'e-teleterm': path.join(__dirname, '/../../../../e/web/teleterm/src'),
        design: path.join(__dirname, '/../../design/src'),
        shared: path.join(__dirname, '/../../shared'),
        'gen-proto-js': path.join(__dirname, '/../../../../gen/proto/js'),
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
