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
  // because of this, the tsconfig.json points to the folder above teleport so
  // code completion works.
  // however this results in vite not being able to find the files when not developing
  // access-graph (or building the production bundle), so we override the `access-graph`
  // package to point to the component that will load access graph from a CDN during runtime

  config.resolve = {
    alias: {
      'access-graph': resolve(__dirname, 'src/AccessGraph/Loader.tsx'),
    },
  };

  return config;
});

export { config as default };
