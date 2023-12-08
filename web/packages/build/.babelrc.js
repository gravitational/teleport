/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
      ['@babel/preset-react', { runtime: 'automatic' }],
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
    ['@babel/preset-react', { runtime: 'automatic' }],
  ];
}

module.exports = {
  env: {
    test: {
      presets: makePresets(true),
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
