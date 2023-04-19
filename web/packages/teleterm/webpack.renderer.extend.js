const path = require('path');

const HtmlWebPackPlugin = require('html-webpack-plugin');
const resolvepath = require('@gravitational/build/webpack/resolvepath');

function extend(cfg) {
  const isDev = cfg.mode === 'development';

  cfg.entry = { app: ['./src/ui/boot'] };
  cfg.output.publicPath = 'auto';
  cfg.output.path = resolvepath('build/app/dist/renderer');
  cfg.output.libraryTarget = 'umd';
  cfg.output.globalObject = 'this';
  cfg.resolve.alias['teleterm'] = path.join(__dirname, './src');
  cfg.plugins = [createHtmlPlugin({ isDev })];

  return cfg;
}

function createHtmlPlugin({ isDev }) {
  const csp = getCsp({ isDev });

  return new HtmlWebPackPlugin({
    filename: 'index.html',
    title: '',
    inject: true,
    templateContent: `
    <!DOCTYPE html>
    <html>
      <head>
        <meta charset="utf-8" />
        <meta http-equiv="X-UA-Compatible" content="IE=edge" />
        <meta name="referrer" content="no-referrer" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <meta http-equiv="Content-Security-Policy" content="${csp}">
      </head>
      <body>
        <div id="app"></div>
      </body>
    </html>`,
  });
}

function getCsp({ isDev }) {
  // feedbackAddress needs to be kept in sync with the address in useShareFeedback.
  const feedbackAddress = isDev
    ? 'https://kcwm2is93l.execute-api.us-west-2.amazonaws.com/prod'
    : 'https://usage.teleport.dev';

  let csp = `
default-src 'self';
connect-src 'self' ${feedbackAddress};
style-src 'self' 'unsafe-inline';
img-src 'self' data: blob:;
object-src 'none';
font-src 'self' data:;
`
    .replaceAll('\n', ' ')
    .trim();

  if (isDev) {
    // Required to make source maps work in dev mode.
    csp += " script-src 'self' 'unsafe-eval';";
  }

  return csp;
}

module.exports = {
  extend,
  createHtmlPlugin,
};
