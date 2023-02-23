/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { arrayObjectIsEqual, mergeDeep } from './highbar';

describe('mergeDeep can merge two', () => {
  it('objects together', () => {
    const a = { a: 1, b: 2, c: 3, e: 5 };
    const b = { a: 3, b: 2, c: 1, d: 4 };
    expect(mergeDeep(a, b)).toStrictEqual({
      a: 3,
      b: 2,
      c: 1,
      d: 4,
      e: 5,
    });
  });

  it('nested objects together', () => {
    const a = { a: 1, b: 2, c: { d: 3, e: 6, g: 8 } };
    const b = { a: 1, b: 2, c: { d: 4, e: 6, f: 7 } };
    expect(mergeDeep(a, b)).toStrictEqual({
      a: 1,
      b: 2,
      c: { d: 4, e: 6, f: 7, g: 8 },
    });
  });

  it('objects together that contain arrays', () => {
    const a = { a: 1, b: ['a', 'b', 'd'] };
    const b = { a: 2, b: ['b', 'c'] };
    expect(mergeDeep(a, b)).toStrictEqual({
      a: 2,
      b: ['b', 'c', 'd'],
    });

    const c = { a: 1, b: ['b', 'c'] };
    const d = { a: 2, b: ['a', 'b', 'd'] };
    expect(mergeDeep(c, d)).toStrictEqual({
      a: 2,
      b: ['a', 'b', 'd'],
    });
  });

  it('objects together that contain arrays of arrays', () => {
    const a = { a: [['b', 'c', 'f']] };
    const b = { a: [['d', 'e']] };
    expect(mergeDeep(a, b)).toStrictEqual({
      a: [['d', 'e', 'f']],
    });

    const c = { a: [['d', 'e']] };
    const d = { a: [['b', 'c', 'f']] };
    expect(mergeDeep(c, d)).toStrictEqual({
      a: [['b', 'c', 'f']],
    });
  });

  it('objects together that contain arrays that contain objects', () => {
    const a = { a: 1, b: [{ c: 3, d: 4, e: 5 }, 'b'] };
    const b = { a: 2, b: [{ c: 3, d: 4, f: 6 }, 'c'] };
    expect(mergeDeep(a, b)).toStrictEqual({
      a: 2,
      b: [{ c: 3, d: 4, e: 5, f: 6 }, 'c'],
    });
  });

  it('objects with arrays with undefined indexes', () => {
    const a = {
      a: false,
      b: {
        c: 'foo',
        d: 'bar',
      },
      c: {
        a: 'no',
        b: [],
      },
    };

    const b = {
      a: true,
      b: {
        d: 'baz',
        e: 'bax',
      },
      c: {
        a: 'ok',
        b: [
          {
            a: 'foo',
            b: 'bar',
          },
        ],
      },
    };
    expect(mergeDeep(a, b)).toStrictEqual({
      a: true,
      b: {
        c: 'foo',
        d: 'baz',
        e: 'bax',
      },
      c: {
        a: 'ok',
        b: [
          {
            a: 'foo',
            b: 'bar',
          },
        ],
      },
    });
  });
});

describe('arrayObjectIsEqual correctly compares', () => {
  it('simple arrays', () => {
    const a = [{ foo: 'bar' }];
    const b = [{ foo: 'bar' }];

    expect(arrayObjectIsEqual(a, b)).toBe(true);

    const c = [{ foo: 'bar' }];
    const d = [{ foo: 'baz' }];
    expect(arrayObjectIsEqual(c, d)).toBe(false);
  });

  it('arrays with complex objects', () => {
    const a = [
      {
        '/clusters/test-uri': {
          accessRequests: {
            pending: {
              app: {},
              db: {},
              kube_cluster: {},
              node: {},
              role: {},
              windows_desktop: {},
            },
            isBarCollapsed: false,
          },
          localClusterUri: '/clusters/test-uri',
          documents: [
            {
              kind: 'doc.cluster',
              title: 'Cluster Test',
              clusterUri: '/clusters/test-uri',
              uri: '/docs/test-cluster-uri',
            },
          ],
          location: '/docs/test-cluster-uri',
          previous: {
            documents: [
              {
                kind: 'doc.terminal_shell',
                uri: '/docs/some_uri',
                title: '/Users/alice/Documents',
              },
            ],
            location: '/docs/some_uri',
          },
        },
      },
    ];

    const b = [
      {
        '/clusters/test-uri': {
          accessRequests: {
            pending: {
              app: {},
              db: {},
              kube_cluster: {},
              node: {},
              role: {},
              windows_desktop: {},
            },
            isBarCollapsed: false,
          },
          localClusterUri: '/clusters/test-uri',
          documents: [
            {
              kind: 'doc.cluster',
              title: 'Cluster Test',
              clusterUri: '/clusters/test-uri',
              uri: '/docs/test-cluster-uri',
            },
          ],
          location: '/docs/test-cluster-uri',
          previous: {
            documents: [
              {
                kind: 'doc.terminal_shell',
                uri: '/docs/some_uri',
                title: '/Users/alice/Documents',
              },
            ],
            location: '/docs/some_uri',
          },
        },
      },
    ];

    expect(arrayObjectIsEqual(a, b)).toBe(true);
  });
});
