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
var pkgs = require('./package.json');

module.exports = {
  entry: {
    app: ['./src/app/index.jsx'],
    vendor: Object.keys(pkgs.dependencies),
    styles: ['./src/styles/bootstrap/bootstrap.scss', './src/styles/inspinia/style.scss', './src/styles/grv.scss']
  },

  devtool: 'source-map',

  output: {
    publicPath: '/web/app',
    path: path.join(__dirname, 'dist/app'),
    filename: '[name].js',
    chunkFilename: '[chunkhash].js',
    sourceMapFilename: '[name].map'
  },

  externals: ['jQuery', 'Terminal', '_' ],

  resolve: {
    root: [ path.join(__dirname, 'src') ],
    extensions: ['', '.js', '.jsx']
  },

  module: {

    loaders: [

      {
        test: /\.svg$/,
        loader: 'svg-sprite'
      },

      { test: /\.(woff|woff2|ttf|eot)$/, loader: "url-loader?limit=10000&name=fonts/[name].[ext]" },
      {
        include: path.join(__dirname, 'src'),
        test: /\.(js|jsx)$/,
        exclude: /node_modules/,
        loader: 'react-hot!babel?loose=all&cacheDirectory!eslint'
      },
      {
        test: /\.scss$/,
        loader: 'style!raw!sass?outputStyle=expanded'
      }
    ]
  },

  plugins: [
    new webpack.DefinePlugin({'process.env.NODE_ENV': '"production"'}),
    new webpack.optimize.OccurenceOrderPlugin(),
    new webpack.optimize.CommonsChunkPlugin({
       names: ['vendor']
     })

    /*new webpack.optimize.UglifyJsPlugin({
     compress: {  warnings: false  }
   })*/
  ]
};
