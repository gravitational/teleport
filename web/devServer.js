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

var fs = require('fs');
var uri = require('url');
var WebpackDevServer = require("webpack-dev-server");
var webpackConfig = require('./webpack/webpack.config.dev.js');
var express = require('express');
var webpack = require('webpack');
var proxy = require('http-proxy').createProxyServer();
var changeProxyResponse = require('./devServerUtils');

// parse target URL 
var argv = require('optimist')
    .usage('Usage: $0 -proxy [url]')
    .demand(['proxy'])
    .argv;

var urlObj = uri.parse(argv.proxy)

if (!urlObj.host) {
  console.error('invalid URL: ' + argv.proxy);
  return;
}

var PROXY_TARGET = urlObj.host;
var ROOT = '/web';
var PORT = '8081';
var WEBPACK_CLIENT_ENTRY = 'webpack-dev-server/client?https://0.0.0.0:' + PORT;
var WEBPACK_SRV_ENTRY = 'webpack/hot/dev-server';

webpackConfig.entry.app.unshift(WEBPACK_CLIENT_ENTRY, WEBPACK_SRV_ENTRY);

function getTargetOptions() {
  return {
    target: "https://"+PROXY_TARGET,
    secure: false,
    changeOrigin: true,
    xfwd: true
  }
}

var compiler = webpack(webpackConfig);

var server = new WebpackDevServer(compiler, {
  proxy:{
    '/web/grafana/*': getTargetOptions(),
    '/web/config.*': getTargetOptions(),
    '/pack/v1/*': getTargetOptions(),
    '/portalapi/*': getTargetOptions(),
    '/portal*': getTargetOptions(),
    '/proxy/*': getTargetOptions(),
    '/v1/*': getTargetOptions(),
    '/app/*': getTargetOptions(),
    '/sites/v1/*': getTargetOptions()
  },
  publicPath: ROOT +'/app',
  hot: true,
  disableHostCheck: true,
  https: true,  
  inline: true,
  headers: { 'X-Custom-Header': 'yes' },
  //stats: { colors: true },
  stats: 'errors-only'
});

// tell webpack dev server to proxy below sockets requests to actual server
server.listeningApp.on('upgrade', function(req, socket) {  
  console.log('proxying ws', req.url);
  proxy.ws(req, socket, {
    target: 'wss://' + PROXY_TARGET,
    secure: false
  });  
});

var htmlToSend = fs.readFileSync(__dirname + "//dist//index.html", 'utf8')

// to enable Hot Module Reload we need to serve local index.html. 
// since local index.html has no embeded TOKEN, we need to:
// 1) make a proxy request
// 2) modify proxy response by replacing server index.html with the local 
// 3) insert embeded by server token into the local
server.app.use(changeProxyResponse(
    (req, res) => {
        // return true if you want to modify the response later
        var contentType = res.getHeader('Content-Type');
        if (contentType && contentType.startsWith('text/html')) {            
            return true;
        }

        return false;
    },
    (req, res, body) => {
        // body is a Buffer with the current response; return Buffer or string with the modified response
        // can also return a Promise.        
        var str = body.toString();      
        htmlToSend = replaceToken(new RegExp(/<meta name="grv_csrf_token" .*\>/), str, htmlToSend);
        htmlToSend = replaceToken(new RegExp(/<meta name="grv_bearer_token" .*\>/), str, htmlToSend);        
        return htmlToSend;
    }
));

function replaceToken(regex, takeFrom, insertTo){
  var value = takeFrom.match(regex);                
  if(value){
    return insertTo.replace(regex, value[0]);        
  }
  return insertTo;
}

server.app.use(ROOT, express.static(__dirname + "//dist"));
server.app.get(ROOT +'/*', function (req, res) {
    proxy.web(req, res,  getTargetOptions());
});

server.listen(PORT, "0.0.0.0", function() {
  console.log('Dev Server is up and running: https://location:' + PORT + '/web/');
});
