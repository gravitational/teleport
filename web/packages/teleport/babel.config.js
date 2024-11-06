import baseCfg from '@gravitational/build/.babelrc';
module.exports = function (api) {
  api.cache(true);
  return baseCfg;
};
