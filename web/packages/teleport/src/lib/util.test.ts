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

import {
  generateTshLoginCommand,
  arrayStrDiff,
  compareByString,
  sortRolesAndFriendlyNames,
} from './util';

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
  expect(arrayStrDiff(null, null)).toStrictEqual([]);

  const arrayA = ['a', 'b', 'c', 'd', 'e'];
  const arrayB = ['b', 'e', 'f', 'g'];

  expect(arrayStrDiff(arrayA, arrayB)).toStrictEqual(['a', 'c', 'd']);
});

test('sortByString with simple string array', () => {
  const arr = ['cats', 'cat', 'x', 'ape', 'apes'];
  expect(arr.sort((a, b) => compareByString(a, b))).toStrictEqual([
    'ape',
    'apes',
    'cat',
    'cats',
    'x',
  ]);
});

test('sortByString with objects with string fields', () => {
  const arr = [
    { name: 'cats', value: 'persian' },
    { name: 'ape', value: 'kingkong' },
    { name: 'cat', value: 'siamese' },
    { name: 'apes', value: 'donkeykong' },
  ];

  expect(arr.sort((a, b) => compareByString(a.name, b.name))).toStrictEqual([
    { name: 'ape', value: 'kingkong' },
    { name: 'apes', value: 'donkeykong' },
    { name: 'cat', value: 'siamese' },
    { name: 'cats', value: 'persian' },
  ]);

  expect(arr.sort((a, b) => compareByString(a.value, b.value))).toStrictEqual([
    { name: 'apes', value: 'donkeykong' },
    { name: 'ape', value: 'kingkong' },
    { name: 'cats', value: 'persian' },
    { name: 'cat', value: 'siamese' },
  ]);
});

test('sortRolesAndFriendlyNames with equal array sizes', () => {
  const roles = ['zrole1', 'crole2', 'brole3'];

  // Should actually be sorted by these display names.
  const friendlyNames = [
    'bfriendly name 1',
    'zfriendly name 2',
    'afriendly name 3',
  ];

  const rolesAndFriendlyNames = sortRolesAndFriendlyNames(roles, friendlyNames);

  expect(rolesAndFriendlyNames.roles).toStrictEqual([
    'brole3',
    'zrole1',
    'crole2',
  ]);

  expect(rolesAndFriendlyNames.friendlyNames).toStrictEqual([
    'afriendly name 3',
    'bfriendly name 1',
    'zfriendly name 2',
  ]);
});

test('sortRolesAndFriendlyNames with equal unequal array sizes', () => {
  const roles = ['zrole1', 'crole2', 'brole3'];

  // Since friendly names isn't the same length as roles, we'll just use roles and
  // ignore thse friendly names.
  const friendlyNames = ['bfriendly name 1', 'zfriendly name 2'];

  const rolesAndFriendlyNames = sortRolesAndFriendlyNames(roles, friendlyNames);

  expect(rolesAndFriendlyNames.roles).toStrictEqual([
    'brole3',
    'crole2',
    'zrole1',
  ]);

  expect(rolesAndFriendlyNames.friendlyNames).toStrictEqual([
    'brole3',
    'crole2',
    'zrole1',
  ]);
});
