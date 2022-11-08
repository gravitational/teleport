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

import { generateTshLoginCommand, arrayStrDiff } from './util';

let windowSpy;

beforeEach(() => {
  windowSpy = jest.spyOn(window, 'window', 'get');
});

afterEach(() => {
  windowSpy.mockRestore();
});

test('with all params defined', () => {
  windowSpy.mockImplementation(() => ({
    location: {
      hostname: 'my-cluster',
      port: '1234',
    },
  }));

  expect(
    generateTshLoginCommand({
      accessRequestId: 'ar-1234',
      username: 'llama',
      authType: 'local',
      clusterId: 'cluster-1234',
    })
  ).toBe(
    'tsh login --proxy=my-cluster:1234 --auth=local --user=llama cluster-1234 --request-id=ar-1234'
  );
});

test('no port and access request id', () => {
  windowSpy.mockImplementation(() => ({
    location: {
      hostname: 'my-cluster',
    },
  }));

  expect(
    generateTshLoginCommand({
      username: 'llama',
      authType: 'sso',
      clusterId: 'cluster-1234',
    })
  ).toBe('tsh login --proxy=my-cluster:443 cluster-1234');
});

test('arrayStrDiff returns the correct diff', () => {
  const arrayA = ['a', 'b', 'c', 'd', 'e'];
  const arrayB = ['b', 'e', 'f', 'g'];

  expect(arrayStrDiff(arrayA, arrayB)).toStrictEqual(['a', 'c', 'd']);
});
