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
var TELEBASE_PATH = '../../web'; 
var TELEBASE_APP_PATH = '../../web/src/app';
var TELEBASE_ASSET_PATH = '../../web/src/assets';

var favIconPath = path.join(ROOT_PATH, TELEBASE_ASSET_PATH+'/img/favicon.ico');
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
    chunkFilename: '[chunkhash].js'    
  },

  noParse: [ /xterm.js$/ ],

  resolve: {

    alias: {      
      'telebase-assets': path.join(ROOT_PATH, TELEBASE_ASSET_PATH),
      'telebase-app': path.join(ROOT_PATH, TELEBASE_APP_PATH),      
      '_': path.join(ROOT_PATH, TELEBASE_ASSET_PATH+'/js/underscore'),
      jquery: path.join(ROOT_PATH, TELEBASE_ASSET_PATH+'/js/jquery'),
      jQuery: path.join(ROOT_PATH, TELEBASE_ASSET_PATH+'/js/jquery')      
    },

    root: [ path.join(ROOT_PATH, 'src')],

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

    extractCss: extractCss,

    handleTelebaseImports: handleTelebaseImports(),

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
      template: TELEBASE_PATH +'/src/index.ejs'
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
    include: [path.join(ROOT_PATH, 'src'), path.join(ROOT_PATH, TELEBASE_PATH)],
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

function handleTelebaseImports() {
  return {
    apply: function (compiler) { 
      compiler.resolvers.normal.apply({
        apply(resolver) {
          resolver.plugin('resolve', (context, request) => {                            
            if (request.path.indexOf('assets/') === 0) {                                
              request.path = request.path.replace('assets/', 'telebase-assets/');                  
            }
            if (context.indexOf('teleport/web/src/app') !== -1) {                
              if (request.path.indexOf('app/') === 0) {                   
                request.path = request.path.replace('app/', 'telebase-app/');                  
              }                                      
            }              
          });
        },
      });
    }
  }
}