const defaultCfg = require('@gravitational/build/webpack/webpack.prod.config');

defaultCfg.module.rules.push({
  test: /proto\/\.js$/,
  loader: 'string-replace-loader',
  options: {
    search: "var global = Function('return this')();",
    replace: 'var global = (function(){ return this }).call(null);',
  },
});

module.exports = defaultCfg;
