const baseCfg = require('@gravitational/build/.babelrc');

baseCfg.presets = [
  ['@babel/preset-env', { targets: { node: 'current' } }],
  ['@babel/preset-react', { runtime: 'automatic' }],
  '@babel/preset-typescript',
];

module.exports = function (api) {
  api.cache(true);
  return baseCfg;
};
