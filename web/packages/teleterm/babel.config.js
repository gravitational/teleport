import baseCfg from '@gravitational/build/.babelrc';
export default function (api) {
  api.cache(true);
  return baseCfg;
}
