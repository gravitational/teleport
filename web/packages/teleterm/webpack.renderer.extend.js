const path = require('path');
const { CleanWebpackPlugin } = require('clean-webpack-plugin');
const HtmlWebPackPlugin = require('html-webpack-plugin');
const resolvepath = require('@gravitational/build/webpack/resolvepath');

function extend(cfg) {
  cfg.entry = { app: ['./src/ui/boot'] };
  cfg.output.publicPath = 'auto';
  cfg.output.path = resolvepath('build/app/dist/renderer');
  cfg.output.libraryTarget = 'umd';
  cfg.output.globalObject = 'this';
  cfg.resolve.alias['teleterm'] = path.join(__dirname, './src');
  cfg.plugins = [new CleanWebpackPlugin(), createHtmlPlugin()];

  return cfg;
}

function createHtmlPlugin() {
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
      </head>
      <body>
        <div id="app"></div>
      </body>
    </html>`,
  });
}

module.exports = {
  extend,
  createHtmlPlugin,
};
