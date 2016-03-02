var path = require('path');
var webpack = require('webpack');
var pkgs = require('./package.json');

module.exports = {
  cache: true,
  devtool: 'inline-source-map',
  entry: {
    app: ['./src/app/index.jsx'],
    vendor: Object.keys(pkgs.dependencies),
    styles: ['./src/styles/bootstrap.scss', './src/styles/inspinia/style.scss', './src/styles/grv-app.scss']
  },

 output: {
    publicPath: '/web/app',
    path: path.join(__dirname, 'dist/app'),
    filename: '[name].js',
    chunkFilename: '[chunkhash].js',
    sourceMapFilename: '[name].map'
  },

  externals: ['jQuery', 'Terminal', 'toastr', '_' ],

  resolve: {
    root: [ path.join(__dirname, 'src') ],
    extensions: ['', '.js', '.jsx']
  },

  module: {

    loaders: [
      { test: /\.(woff|woff2|ttf|eot|svg)$/,  loader: "url-loader?limit=10000&name=fonts/[name].[ext]" },
      {
        include: path.join(__dirname, 'src'),
        test: /\.js$/,
        exclude: /node_modules/,
        loader: 'eslint!babel?loose=all&cacheDirectory'
      },
      {
        include: path.join(__dirname, 'src'),
        test: /\.jsx$/,
        exclude: /node_modules/,
        loader: 'react-hot!babel?loose=all&cacheDirectory'
      },
      {
        test: /\.scss$/,
        loader: 'style!raw!sass?outputStyle=expanded'
      }
    ]
  },

  node: {
    Buffer: true
  },

  plugins: [
    new webpack.optimize.OccurenceOrderPlugin(),
    new webpack.optimize.CommonsChunkPlugin({
       names: ['vendor']
     })
  ]
};
