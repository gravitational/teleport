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
var HtmlWebPackPlugin = require('html-webpack-plugin');
var ExtractTextPlugin = require('extract-text-webpack-plugin');
var ROOT_PATH = path.join(__dirname, '../');
var FlowBabelWebpackPlugin = require('flow-babel-webpack-plugin');
var favIconPath = path.join(ROOT_PATH, 'src/assets/img/favicon.ico');
var extractCss = new ExtractTextPlugin('vendor.[contenthash].css');

module.exports = {

  entry: {
    app: ['./src/app/index.jsx'],
    vendor: ['./src/app/vendor'],
    styles: ['./src/styles/grv.scss']
  },

  output: {
    publicPath: '/web/app',
    path: path.join(ROOT_PATH, 'dist/app'),
    filename: '[name].[hash].js',
    chunkFilename: '[chunkhash].js',
    sourceMapFilename: '[name].map'
  },

  resolve: {

    alias: {
      '_': path.join(ROOT_PATH, 'src/assets/js/underscore'),
      jquery: path.join(ROOT_PATH, 'src/assets/js/jquery'),
      jQuery: path.join(ROOT_PATH, 'src/assets/js/jquery'),
      Terminal: path.join(ROOT_PATH, 'src/assets/js/terminal')
    },

    root: [ path.join(ROOT_PATH, 'src') ],

    extensions: ['', '.js', '.jsx']
  },

  loaders: {

    svg: {
      test: /\.svg$/,
      loader: 'svg-sprite'
    },

    fonts: {
      test: /fonts\/(.)+\.(woff|woff2|ttf|eot|svg)/,
      loader: "url-loader?limit=10000&name=/assets/fonts/[name].[ext]"
    },

    images: {
      test: /\.(png|jpg|gif)$/,
      loader: "file-loader?name=/assets/img/img-[hash:6].[ext]"
    },

    js: js,

    scss: {
      test: /\.scss$/,
      loader: 'style!css!sass?outputStyle=expanded'
    },

    css: {
      test: /\.scss$/,
      loader: extractCss.extract(['css','sass'])
    }
  },

  plugins: {
  
    flowType: new FlowBabelWebpackPlugin(),

    extractCss: extractCss,

    hotReplacement: new webpack.HotModuleReplacementPlugin(),

    devBuild: new webpack.DefinePlugin({ 'process.env.NODE_ENV': JSON.stringify('development') }),

    releaseBuild: new webpack.DefinePlugin({ 'process.env.NODE_ENV': JSON.stringify('production') }),

    testBuild: new webpack.DefinePlugin({ 'process.env.NODE_ENV': JSON.stringify('test') }),

    vendorBundle: new webpack.optimize.CommonsChunkPlugin({
       names: ['vendor']
    }),

    createIndexHtml: new HtmlWebPackPlugin({
      filename: '../index.html',
      favicon: favIconPath,
      title: 'Teleport by Gravitational',
      inject: true,
      template: 'src/index.ejs'
    }),

    uglify: uglify
   }
};

function js(args){
  args = args || {};
  var loader = 'babel?cacheDirectory!eslint';
  if(args.withHot){
    loader = 'react-hot!' + loader;
  }

  return {
    include: path.join(ROOT_PATH, 'src'),
    test: /\.(js|jsx)$/,
    exclude: /(node_modules)|(assets)/,
    loader: loader
  }
}

function uglify(args){
  args = args || {};

  var props = {
    compress: {  warnings: false  }
  }

  if(args.onlyVendor){
    props.include = /vendor/;
  }

  return new webpack.optimize.UglifyJsPlugin(props)
}
