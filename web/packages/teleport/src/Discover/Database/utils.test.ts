/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { matchLabels } from './util';

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
