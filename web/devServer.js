var WebpackDevServer = require("webpack-dev-server");
var webpackConfig = require('./webpack.config.js');
var express = require('express');
var webpack = require('webpack');

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
    //"v1/webapi/users/invites": "http://x220:3080"
    /*"/v1/socket/*" : {
      target: 'ws://localhost:3080/v1/webapi/',
      ws: true
    },*/
    "/v1/webapi/*": {
      target: "http://0.0.0.0:3080/"
    },
    "/socket/*": {
      target: "ws://0.0.0.0:3080/v1/",
      ws: true
    }
    ///"/sites/v1/*": "http://172.28.128.4:34444"
  },
  publicPath: ROOT +'/app',
  hot: true,
  inline: true,
  headers: { "X-Custom-Header": "yes" },
  stats: 'errors-only'
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
