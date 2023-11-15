/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
