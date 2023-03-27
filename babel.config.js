const baseCfg = require('@gravitational/build/.babelrc');
module.exports = function (api) {
  api.cache(true);
  return baseCfg;
};
