/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

/* eslint-disable no-console */

const WebpackDevServer = require('webpack-dev-server');
const httpProxy = require('http-proxy');
const optimist = require('optimist');

const modifyIndexHtmlMiddleware = require('./modifyResponse');
const initCompiler = require('./initCompiler');

// parse target URL
const argv = optimist
  .usage('Usage: $0 -target [url] -config [config]')
  .demand(['target', 'config']).argv;

const target = argv.target.startsWith('https')
  ? argv.target
  : `https://${argv.target}`;
const urlObj = new URL(target);
const webpackConfig = require(argv.config);

if (!urlObj.host) {
  console.error('invalid URL: ' + target);
  return;
}

const PROXY_TARGET = urlObj.host;
const ROOT = '/web';
const PORT = process.env.WEBPACK_PORT || 8080;

// init webpack compiler
const compiler = initCompiler({ webpackConfig });

compiler.callWhenReady(function () {
  console.log(
    '\x1b[33m',
    `DevServer is ready to serve: https://localhost:${PORT}/web/`,
    '\x1b[0m'
  );
});

function getTargetOptions() {
  return {
    target: 'https://' + PROXY_TARGET,
    secure: false,
    changeOrigin: true,
    xfwd: true,
  };
}

function getWebpackDevServerConfig() {
  const config = {
    proxy: [
      {
        ...getTargetOptions(),
        context: function (pathname, req) {
          // proxy requests to /web/config*
          if (/^\/web\/config/.test(pathname)) {
            return true;
          }

          // proxy requests to /v1/*
          if (/^\/v1\//.test(pathname)) {
            return true;
          }

          if (!process.env.WEBPACK_PROXY_APP_ACCESS) {
            return false;
          }

          // Proxy requests to any hostname that does not match the proxy hostname
          // This is to make application access work:
          // - When proxying to https://go.teleport, we want to serve Webpack for
          //   those requests.
          // - When handling requests for https://dumper.go.teleport, we want to proxy
          //   all requests through Webpack to that application
          const { hostname } = new URL('https://' + req.headers.host);

          return hostname !== urlObj.hostname;
        },
      },
    ],
    static: {
      serveIndex: false,
      publicPath: ROOT + '/app',
    },
    server: {
      type: 'https',
    },
    host: '0.0.0.0',
    port: PORT,
    allowedHosts: 'all',
    client: {
      overlay: false,
      webSocketURL: 'auto://0.0.0.0:0/ws',
    },
    webSocketServer: 'ws',
    devMiddleware: {
      stats: 'minimal',
    },
    hot: true,
    headers: {
      'X-Custom-Header': 'yes',
    },
  };

  const cert = process.env.WEBPACK_HTTPS_CERT;
  const key = process.env.WEBPACK_HTTPS_KEY;
  const ca = process.env.WEBPACK_HTTPS_CA;
  const pfx = process.env.WEBPACK_HTTPS_PFX;
  const passphrase = process.env.WEBPACK_HTTPS_PASSPHRASE;

  // we need either cert + key, or the pfx file
  if ((cert && key) || pfx) {
    config.server.options = {
      cert,
      key,
      ca,
      pfx,
      passphrase,
    };
  }

  return config;
}

const devServer = new WebpackDevServer(
  getWebpackDevServerConfig(),
  compiler.webpackCompiler
);

// create a dedicated proxy server to proxy cherry-picked requests
// to the remote target
const proxyServer = httpProxy.createProxyServer();
process.on('SIGINT', () => {
  proxyServer.close();
});

// serveIndexHtml proxies all requests skipped by webpack-dev-server to
// targeted server, these are requests to index.html (app entry point)
function serveIndexHtml(req, res) {
  // prevent gzip compression so it's easier for us to parse the original response
  // to retrieve tokens (csrf and access tokens)
  if (req.headers['accept-encoding']) {
    req.headers['accept-encoding'] = req.headers['accept-encoding']
      .replace('gzip, ', '')
      .replace(', gzip,', ',')
      .replace('gzip', '');
  }

  function handleRequest() {
    proxyServer.web(req, res, getTargetOptions(), (err, req, res) => {
      const msg = `error handling request: ${err.message}. Is the target running and accessible at ${target}?`;
      console.error(msg);
      res.write(msg);
      res.end();
    });
  }

  if (!compiler.isLocalIndexHtmlReady()) {
    compiler.callWhenReady(handleRequest);
  } else {
    handleRequest();
  }
}

devServer.start().then(() => {
  devServer.app.use(modifyIndexHtmlMiddleware(compiler));
  devServer.app.get('/*', serveIndexHtml);
  devServer.server.on('upgrade', (req, socket) => {
    if (req.url === '/ws') {
      // webpack WS (hot reloads endpoint)
      return;
    }
    console.log('proxying ws', req.url);
    proxyServer.ws(req, socket, {
      target: 'wss://' + PROXY_TARGET,
      secure: false,
    });
    proxyServer.on('error', err => {
      console.error('ws error:', err);
    });
  });
});
