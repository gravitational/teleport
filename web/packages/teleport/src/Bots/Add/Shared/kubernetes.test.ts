/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { makeKubernetesAccessChecker } from './kubernetes';

describe('prepareKubernetesAccessChecker', () => {
  test('zero labels', async () => {
    const checker = makeKubernetesAccessChecker([]);

    expect(checker.check([])).toBeFalsy();
    expect(checker.check([{ name: '*', value: '*' }])).toBeFalsy();
  });

  describe('literals', () => {
    test('single', async () => {
      const checker = makeKubernetesAccessChecker([
        {
          name: 'foo',
          values: ['bar'],
        },
      ]);

      expect(checker.check([])).toBeFalsy();
      expect(checker.check([{ name: 'foo', value: 'baz' }])).toBeFalsy();
      expect(checker.check([{ name: 'foo', value: 'bar' }])).toBeTruthy();
    });

    test('multiple', async () => {
      const checker = makeKubernetesAccessChecker([
        {
          name: 'foo',
          values: ['bar', 'baz'],
        },
      ]);

      expect(checker.check([])).toBeFalsy();
      expect(checker.check([{ name: 'foo', value: 'baz' }])).toBeTruthy();
      expect(checker.check([{ name: 'foo', value: 'bar' }])).toBeTruthy();
      expect(checker.check([{ name: 'foo', value: 'box' }])).toBeFalsy();
    });
  });

  describe('wildcards', () => {
    test('single', async () => {
      const checker = makeKubernetesAccessChecker([
        {
          name: 'foo',
          values: ['ba*'],
        },
      ]);

      expect(checker.check([])).toBeFalsy();
      expect(checker.check([{ name: 'foo', value: 'baz' }])).toBeTruthy();
      expect(checker.check([{ name: 'foo', value: 'bar' }])).toBeTruthy();
      expect(checker.check([{ name: 'foo', value: 'box' }])).toBeFalsy();
    });

    test('multiple', async () => {
      const checker = makeKubernetesAccessChecker([
        {
          name: 'foo',
          values: ['bar*', 'baz', 'b*x'],
        },
      ]);

      expect(checker.check([])).toBeFalsy();
      expect(checker.check([{ name: 'foo', value: 'baz' }])).toBeTruthy();
      expect(checker.check([{ name: 'foo', value: 'bar' }])).toBeTruthy();
      expect(checker.check([{ name: 'foo', value: 'box' }])).toBeTruthy();
      expect(checker.check([{ name: 'foo', value: 'bot' }])).toBeFalsy();
    });

    test('special', async () => {
      const checker = makeKubernetesAccessChecker([
        {
          name: '*',
          values: ['*'],
        },
        {
          name: 'never',
          values: ['never'],
        },
      ]);

      expect(checker.check([])).toBeTruthy();
      expect(checker.check([{ name: 'foo', value: 'bar' }])).toBeTruthy();
      expect(checker.check([{ name: 'bar', value: 'baz' }])).toBeTruthy();
    });
  });

  describe('regex', () => {
    test('single', async () => {
      const checker = makeKubernetesAccessChecker([
        {
          name: 'foo',
          values: ['^ba(z|r)$'],
        },
      ]);

      expect(checker.check([])).toBeFalsy();
      expect(checker.check([{ name: 'foo', value: 'baz' }])).toBeTruthy();
      expect(checker.check([{ name: 'foo', value: 'bar' }])).toBeTruthy();
      expect(checker.check([{ name: 'foo', value: 'box' }])).toBeFalsy();
    });
    test('multiple', async () => {
      const checker = makeKubernetesAccessChecker([
        {
          name: 'foo',
          values: ['^ba(z|r)$', '^bo(x|y)$'],
        },
      ]);

      expect(checker.check([])).toBeFalsy();
      expect(checker.check([{ name: 'foo', value: 'baz' }])).toBeTruthy();
      expect(checker.check([{ name: 'foo', value: 'bar' }])).toBeTruthy();
      expect(checker.check([{ name: 'foo', value: 'box' }])).toBeTruthy();
      expect(checker.check([{ name: 'foo', value: 'bot' }])).toBeFalsy();
    });
  });

  test('combination', async () => {
    const checker = makeKubernetesAccessChecker([
      {
        name: 'region',
        values: ['us-west-*'],
      },
      {
        name: 'env',
        values: ['^(dev|staging)-[0-9]+$'],
      },
      {
        name: 'cloud',
        values: ['aws', 'gcp'],
      },
    ]);

    expect(checker.check([])).toBeFalsy();
    expect(checker.check([{ name: 'region', value: 'us-west-1' }])).toBeFalsy();
    expect(checker.check([{ name: 'env', value: 'dev-01' }])).toBeFalsy();
    expect(checker.check([{ name: 'cloud', value: 'gcp' }])).toBeFalsy();
    expect(
      checker.check([
        { name: 'region', value: 'us-west-1' },
        { name: 'env', value: 'dev-01' },
        { name: 'cloud', value: 'gcp' },
      ])
    ).toBeTruthy();
    expect(
      checker.check([
        { name: 'region', value: 'eu-west-1' },
        { name: 'env', value: 'dev-01' },
        { name: 'cloud', value: 'gcp' },
      ])
    ).toBeFalsy();
    expect(
      checker.check([
        { name: 'region', value: 'us-west-1' },
        { name: 'env', value: 'staging-abc' },
        { name: 'cloud', value: 'gcp' },
      ])
    ).toBeFalsy();
    expect(
      checker.check([
        { name: 'region', value: 'us-west-1' },
        { name: 'env', value: 'dev-01' },
        { name: 'cloud', value: 'azure' },
      ])
    ).toBeFalsy();
  });
});
