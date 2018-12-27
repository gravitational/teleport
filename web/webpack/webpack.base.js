/*
Copyright 2018 Gravitational, Inc.

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
var MiniCssExtractPlugin = require("mini-css-extract-plugin");
var HtmlWebPackPlugin = require('html-webpack-plugin');

var ROOT_PATH = path.join(__dirname, '../');
var FAVICON_PATH = path.join(ROOT_PATH, 'src/assets/img/favicon.ico');

module.exports = {

  entry: {
    app: ['./src/boot.js'],
  },

  optimization: {
    splitChunks: {
      cacheGroups: {
        vendors: {
          chunks: "all",
          name: "vendor",
          test: /([\\/]node_modules[\\/])|(assets\/)/,
          priority: -10
        }
      }
    }
  },

  output: {
    // used by loaders to generate various URLs within CSS, JS based off publicPath
    publicPath: '/web/app',

    path: path.join(ROOT_PATH, 'dist/app'),

    /*
    * format of the output file names. [name] stands for 'entry' keys
    * defined in the 'entry' section
    **/
    filename: '[name].[hash].js',

    // chunk file name format
    chunkFilename: '[name].[chunkhash].js'
  },

  resolve: {
    // some vendor libraries expect below globals to be defined
    alias: {
      jquery: path.join(ROOT_PATH, '/src/assets/js/jquery'),
      jQuery: path.join(ROOT_PATH, '/src/assets/js/jquery'),
      app: path.join(ROOT_PATH, '/src/app'),
      assets: path.join(ROOT_PATH, '/src/assets/'),
    },

    modules: ['node_modules'],

    extensions: ['.js', '.jsx']
  },

  noParse: function(content) {
    return /xterm.js$/.test(content);
  },

  rules: {
    fonts: {
      test: /fonts\/(.)+\.(woff|woff2|ttf|eot|svg)/,
      loader: "url-loader",
      options: {
        limit: 10000,
        name: '/assets/fonts/[name].[ext]',
      }
    },

    svg: {
      test: /\.svg$/,
      loader: 'svg-sprite-loader',
      exclude: /node_modules/
    },

    css({ dev } = {}){
      var use = []
      if (dev) {
        use = ['style-loader', 'css-loader'];
      } else {
        use = [MiniCssExtractPlugin.loader, 'css-loader']
      }

      return {
        test: /\.(css)$/,
        use: use
      }
    },

    scss({ dev } = {})
    {
      var sassLoader = {
        loader: 'sass-loader',
        options: {
          outputStyle: "compressed",
          precision: 9
        } };

      var use = []
      if (dev) {
        use = ['style-loader', 'css-loader', sassLoader];
      } else {
        use = [MiniCssExtractPlugin.loader, 'css-loader', sassLoader]
      }

      return {
        test: /\.(scss)$/,
        use: use
      }
    },

    inlineStyle: {
      test: /\.scss$/,
      use: ['style-loader', 'css-loader', 'sass-loader']
    },

    images: {
      test: /\.(png|jpg|gif)$/,
      loader: "file-loader",
      options: {
        limit: 10000,
        name: '/assets/img/img-[hash:6].[ext]',
      }
    },

    jsx: jsx,
    jslint: {
      enforce: "pre",
      test: /\.(js)|(jsx)$/,
      exclude: /(node_modules)|(.json$)|(assets)/,
      loader: "eslint-loader",
    },
  },

  plugins: {

    // builds index html page, the main entry point for application
    createIndexHtml() {
      return createHtmlPluginInstance({
        filename: '../index.html',
        favicon: FAVICON_PATH,
        title: '',
        inject: true,
        template: 'src/index.ejs'
      })
    },

    // extracts all vendor styles and puts them into separate css file
    extractAppCss() {
      return new MiniCssExtractPlugin({
        filename: "styles.[contenthash].css",
      })
    }
  }
};

function jsx(args){
  args = args || {};
  var plugins = ["transform-class-properties", "transform-object-rest-spread", "syntax-dynamic-import"];
  var moduleType = false;
  var emitWarning = false;

  if(args.withHot){
    plugins.unshift('react-hot-loader/babel');
    emitWarning = true;
  }

  // use commonjs modules to be able to override exports in tests
  if(args.test){
    moduleType = 'commonjs'
  }

  var presets =   ['react', [ "es2015", { "modules": moduleType } ] ];

  return {
    include: [path.join(ROOT_PATH, 'src')],
    test: /\.(js|jsx)$/,
    exclude: /(node_modules)|(assets)/,
    use: [
      {
        loader: 'babel-loader',
        options: {
          presets,
          plugins,
          // This is a feature of `babel-loader` for webpack (not Babel itself).
          // It enables caching results in ./node_modules/.cache/babel-loader/
          // directory for faster rebuilds.
          cacheDirectory: true,
        }
      },
      {
        loader: "eslint-loader",
        options: {
          emitWarning,
        }
      }
    ]
  }
}

function createHtmlPluginInstance(cfg) {
  cfg.inject = true;
  return new HtmlWebPackPlugin(cfg)
}