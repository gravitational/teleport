/**
 * Copyright 2022 Gravitational, Inc.
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

import { hasMatchingLabels } from './DownloadScript';

const dbLabels = [
  { name: 'os', value: 'mac' },
  { name: 'env', value: 'prod' },
  { name: 'tag', value: 'v11.0.0' },
];

const testCases = [
  {
    name: 'match with exact values',
    dbLabels: dbLabels,
    agentLabels: [
      { name: 'tag', value: 'v11.0.0' }, // match
      { name: 'os', value: 'linux' },
      { name: 'os', value: 'windows' },
      { name: 'env', value: 'prod' }, // match
      { name: 'env', value: 'dev' },
      { name: 'os', value: 'mac' }, // match
      { name: 'tag', value: 'v12.0.0' },
    ],
    expectedMatch: true,
  },
  {
    name: 'match by all asteriks',
    dbLabels: dbLabels,
    agentLabels: [
      { name: 'fruit', value: 'apple' },
      { name: '*', value: '*' },
    ],
    expectedMatch: true,
  },
  {
    name: 'match by all asteriks, with no dbLabels defined',
    dbLabels: [],
    agentLabels: [
      { name: 'fruit', value: 'apple' },
      { name: '*', value: '*' },
    ],
    expectedMatch: true,
  },
  {
    name: 'match by key asteriks',
    dbLabels: dbLabels,
    agentLabels: [
      { name: 'os', value: '*' },
      { name: 'env', value: '*' },
      { name: 'tag', value: '*' },
    ],
    expectedMatch: true,
  },
  {
    name: 'match by value asteriks',
    dbLabels: dbLabels,
    agentLabels: [
      { name: '*', value: 'prod' },
      { name: '*', value: 'mac' },
      { name: '*', value: 'v11.0.0' },
    ],
    expectedMatch: true,
  },
  {
    name: 'match by asteriks and exacts',
    dbLabels: dbLabels,
    agentLabels: [
      { name: 'os', value: 'windows' },
      { name: '*', value: 'prod' },
      { name: 'os', value: '*' },
      { name: 'tag', value: 'v11.0.0' },
      { name: 'tag', value: 'v12.0.0' },
      { name: '*', value: 'banana' },
    ],
    expectedMatch: true,
  },
  {
    name: 'no match despite having all matching labels',
    dbLabels: dbLabels,
    agentLabels: [
      ...dbLabels,
      { name: 'fruit', value: 'banana' }, // the culprit
    ],
  },
  {
    name: 'no matches',
    dbLabels: dbLabels,
    agentLabels: [{ name: 'fruit', value: 'banana' }],
  },
  {
    name: 'no matches with empty agentLabels list',
    dbLabels: dbLabels,
    agentLabels: [],
  },
];

test.each(testCases)('$name', ({ dbLabels, agentLabels, expectedMatch }) => {
  const match = hasMatchingLabels(dbLabels, agentLabels);
  expect(match).toEqual(Boolean(expectedMatch));
});
