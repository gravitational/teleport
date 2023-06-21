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

function makePresets(test = false) {
  if (test) {
    return [
      ['@babel/preset-env', { targets: { node: 'current' } }],
      '@babel/preset-react',
      '@babel/preset-typescript',
    ];
  }

  return [
    [
      '@babel/preset-env',
      {
        targets:
          'last 2 chrome version, last 2 edge version, last 2 firefox version, last 2 safari version',
      },
    ],
    '@babel/preset-react',
  ];
}

module.exports = {
  env: {
    test: {
      presets: makePresets(true),
      plugins: ['babel-plugin-transform-import-meta'],
    },
    development: {
      plugins: [
        ...plugins,
        ['babel-plugin-styled-components', { displayName: true, ssr: false }],
      ],
    },
    production: {
      plugins: [
        ...plugins,
        ['babel-plugin-styled-components', { displayName: false, ssr: false }],
      ],
    },
  },
  presets: makePresets(),
  plugins: [
    ...plugins,
    ['babel-plugin-styled-components', { displayName: false, ssr: false }],
  ],
};
