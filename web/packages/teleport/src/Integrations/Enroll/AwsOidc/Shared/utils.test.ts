/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import cfg from 'teleport/config';

import {
  getDefaultS3BucketName,
  getDefaultS3PrefixName,
  requiredPrefixName,
  requiredBucketName,
} from './utils';

const defaultProxyCluster = cfg.proxyCluster;

afterEach(() => {
  cfg.proxyCluster = defaultProxyCluster;
});

test('getDefaultS3BucketName', () => {
  cfg.proxyCluster = 'llama#42.slkej';
  expect(getDefaultS3BucketName()).toBe('');

  cfg.proxyCluster = 'llama.cloud.gravitational-io';
  expect(getDefaultS3BucketName()).toBe('llama-cloud-gravitational-io');
});

test('getDefaultS3PrefixName', () => {
  expect(getDefaultS3PrefixName('')).toBe('');
  expect(getDefaultS3PrefixName('sdf@$@#sdf')).toBe('');

  cfg.proxyCluster = 'llama.cloud.gravitational-io';
  expect(getDefaultS3PrefixName('int-name')).toBe('int-name-oidc-idp');
});

describe('requiredPrefixName', () => {
  const requiredField = true;
  test.each`
    input                         | valid
    ${''}                         | ${false}
    ${Array.from('x'.repeat(64))} | ${false}
    ${'-sdf'}                     | ${false}
    ${'sdfs-'}                    | ${false}
    ${'_sdf'}                     | ${false}
    ${'sdfd_'}                    | ${false}
    ${'..sdf'}                    | ${false}
    ${'sdf.'}                     | ${false}
    ${'sdlfkjs/dfsd'}             | ${false}
    ${'Asd09f-_.sdfDFs1'}         | ${true}
  `('validity of input($input) should be ($valid)', ({ input, valid }) => {
    const result = requiredPrefixName(requiredField)(input)();
    expect(result.valid).toEqual(valid);
  });

  test('empty prefix name is valid if not a required field', () => {
    const requiredField = false;
    expect(requiredPrefixName(requiredField)('')().valid).toBeTruthy();
  });
});

describe('requiredBucketName', () => {
  test.each`
    input                         | valid
    ${''}                         | ${false}
    ${Array.from('x'.repeat(64))} | ${false}
    ${Array.from('x'.repeat(2))}  | ${false}
    ${'-sdf'}                     | ${false}
    ${'sdfs-'}                    | ${false}
    ${'sdfds_sdf'}                | ${false}
    ${'xn--sdf'}                  | ${false}
    ${'sthree-sdf'}               | ${false}
    ${'sthree-configurator-dfs'}  | ${false}
    ${'sdf-s3alias'}              | ${false}
    ${'sdf--ol-s3'}               | ${false}
    ${'Asd09f-sdfDFs1'}           | ${false}
    ${'sdf0-dfs0'}                | ${true}
  `('validity of input($input) should be ($valid)', ({ input, valid }) => {
    const requiredField = true;
    const result = requiredBucketName(requiredField)(input)();
    expect(result.valid).toEqual(valid);
  });

  test('empty bucket name is valid if not a required field', () => {
    const requiredField = false;
    expect(requiredBucketName(requiredField)('')().valid).toBeTruthy();
  });
});
