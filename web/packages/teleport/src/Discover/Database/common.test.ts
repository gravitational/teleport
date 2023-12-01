/**
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

import { matchLabels } from './common';

const newDbLabels = [
  { name: 'env', value: 'prod' },
  { name: 'os', value: 'mac' },
  { name: 'tag', value: 'v11.0.0' },
];

const testCases = [
  {
    name: 'match multiple by exact keys and values',
    match: true,
    matcherLabels: {
      env: ['prod'],
      os: ['mac'],
      tag: ['v11.0.0'],
    },
  },
  {
    name: 'match by multivalues',
    match: true,
    matcherLabels: {
      env: ['prod', 'staging'],
      os: ['windows', 'mac', 'linux'],
      tag: ['1', '2', '3', 'v11.0.0'],
    },
  },
  {
    name: 'match with one label set',
    match: true,
    matcherLabels: {
      tag: ['v1', 'v11.0.0', 'v2'],
    },
  },
  {
    name: 'match by asteriks',
    match: true,
    matcherLabels: {
      env: ['na'],
      os: ['na'],
      '*': ['*'],
    },
  },
  {
    name: 'match with a key and value asterik',
    match: true,
    matcherLabels: {
      '*': ['prod', 'staging'],
      os: ['*'],
    },
  },
  {
    name: 'match by asteriks with no db labels defined',
    noDbLabels: true,
    match: true,
    matcherLabels: { '*': ['*'] },
  },
  {
    name: 'no match with no db labels defined and no matcher labels',
    noDbLabels: true,
    match: false,
    matcherLabels: {},
  },
  {
    name: 'no match with no db labels with matcher labels',
    noDbLabels: true,
    match: false,
    matcherLabels: { os: ['mac'] },
  },
  {
    name: 'no match despite other matching labels',
    match: false,
    matcherLabels: {
      '*': ['no-match'], // no match
      env: ['prod'],
      os: ['mac'],
      tag: ['v11.0.0'],
    },
  },
  {
    name: 'no match with empty labels',
    match: false,
    matcherLabels: {},
  },
  {
    name: 'no match with empty label values',
    match: false,
    matcherLabels: { os: [] },
  },
];

describe('matchLabels()', () => {
  test.each(testCases)('$name', ({ matcherLabels, match, noDbLabels }) => {
    const isMatched = matchLabels(noDbLabels ? [] : newDbLabels, matcherLabels);
    expect(isMatched).toEqual(match);
  });
});
