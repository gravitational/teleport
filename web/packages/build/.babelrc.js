/*
Copyright 2019 Gravitational, Inc.

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

const plugins = [
  '@babel/plugin-proposal-class-properties',
  '@babel/plugin-proposal-object-rest-spread',
  '@babel/plugin-proposal-optional-chaining',
  '@babel/plugin-syntax-dynamic-import',
];

function makePresents(test = false) {
  const presents = ['@babel/preset-react', '@babel/preset-typescript'];

  if (test) {
    return [
      ['@babel/preset-env', { targets: { node: 'current' } }],
      ...presents,
    ];
  }

  return ['@babel/preset-env', ...presents];
}

module.exports = {
  env: {
    test: {
      presets: makePresents(true),
    },
    development: {
      plugins: [
        ...plugins,
        ['babel-plugin-styled-components', { displayName: true, ssr: false }],
      ],
    },
  },
  presets: makePresents(),
  plugins: [
    ...plugins,
    ['babel-plugin-styled-components', { displayName: false, ssr: false }],
  ],
};
