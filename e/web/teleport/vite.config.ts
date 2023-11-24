import { resolve } from 'path';

import { defineConfig } from 'vite';

import { createViteConfig } from '../../../web/packages/build/vite/config';

const rootDirectory = resolve(__dirname, '../../..');
const outputDirectory = resolve(rootDirectory, 'webassets/e/teleport');

const config = defineConfig(env => {
  const config = createViteConfig(rootDirectory, outputDirectory)(env);

  // when developing access graph, the access graph code lives above teleport,
  // like so: -
  // access-graph/
  //   teleport/
  //     e/
  // in order to have a point where access-graph and teleport can point to,
  // we use a module called `access-graph`.
  // when building teleport, this points to the code that loads it from an external CDN (production).
  // when developing access-graph, the acces-graph Vite config changes this to point to the source code.

  config.resolve = {
    alias: {
      'access-graph': resolve(__dirname, 'src/AccessGraph/Loader.tsx'),
    },
  };

  return config;
});

export { config as default };
