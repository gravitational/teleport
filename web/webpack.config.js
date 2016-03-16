var path = require('path');
var webpack = require('webpack');
var pkgs = require('./package.json');

module.exports = {
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

  externals: ['jQuery', 'Terminal', '_' ],

  resolve: {
    root: [ path.join(__dirname, 'src') ],
    extensions: ['', '.js', '.jsx']
  },

  module: {

    loaders: [
      { test: /\.(woff|woff2|ttf|eot|svg)$/, loader: "url-loader?limit=10000&name=fonts/[name].[ext]" },
      {
        include: path.join(__dirname, 'src'),
        test: /\.(js|jsx)$/,
        exclude: /node_modules/,
        loader: 'eslint!react-hot!babel?loose=all&cacheDirectory'
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
     }),

    new webpack.optimize.UglifyJsPlugin({
     compress: {  warnings: false  }
   })
  ]
};
