var WebpackDevServer = require("webpack-dev-server");
var webpackConfig = require('./webpack.config.js');
var express = require('express');
var webpack = require('webpack');
var proxy = require('http-proxy').createProxyServer();

var PROXY_TARGET = '0.0.0.0:3080/';
var ROOT = '/web';
var PORT = '8080';
var WEBPACK_CLIENT_ENTRY = 'webpack-dev-server/client?http://localhost:' + PORT;
var WEBPACK_SRV_ENTRY = 'webpack/hot/dev-server';

webpackConfig.plugins.unshift(new webpack.HotModuleReplacementPlugin());
webpackConfig.entry.app.unshift(WEBPACK_CLIENT_ENTRY, WEBPACK_SRV_ENTRY);
webpackConfig.entry.styles.unshift(WEBPACK_CLIENT_ENTRY, WEBPACK_SRV_ENTRY);

var compiler = webpack(webpackConfig);

var server = new WebpackDevServer(compiler, {
  proxy: {
    '/v1/webapi/*': {
      target: 'http://' + PROXY_TARGET
    }
  },
  publicPath: ROOT +'/app',
  hot: true,
  inline: true,
  headers: { 'X-Custom-Header': 'yes' },
  //stats: 'errors-only'
  stats: { colors: true },
});

// tell webpack dev server to proxy below sockets requests to actual server
server.listeningApp.on('upgrade', function(req, socket) {
  if (req.url.match('/v1/webapi/sites')) {
    console.log('proxying ws', req.url);
    proxy.ws(req, socket, {'target': 'ws://' + PROXY_TARGET });
  }
});

server.app.use(ROOT, express.static(__dirname + "//dist"));
server.app.get(ROOT +'/*', function (req, res) {
    res.sendfile(__dirname + "//dist//index.html");
});

module.exports = function(){
  server.listen(PORT, "localhost", function() {
    console.log('Dev Server is up and running: http://location:' + PORT);
  });
}
