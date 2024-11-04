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

import { arrayObjectIsEqual, equalsDeep, mergeDeep } from './highbar';

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

describe('equalsDeep', () => {
  describe.each([false, true])('with ignoreUndefined=%s', ignoreUndefined => {
    it('compares primitive values', () => {
      expect(equalsDeep(1, 2, { ignoreUndefined })).toBe(false);
      expect(equalsDeep(2, 2, { ignoreUndefined })).toBe(true);
    });

    it('compares simple objects', () => {
      expect(equalsDeep({}, {}, { ignoreUndefined })).toBe(true);
      expect(
        equalsDeep({ a: 'b', c: 4 }, { c: 5, a: 'b' }, { ignoreUndefined })
      ).toBe(false);
      expect(
        equalsDeep({ a: 'b', c: 4 }, { c: 4, a: 'b' }, { ignoreUndefined })
      ).toBe(true);

      // Corner case: falsy values
      expect(
        equalsDeep({}, { a: 0, b: false, c: '' }, { ignoreUndefined })
      ).toBe(false);
    });

    it('compares complex objects', () => {
      expect(
        equalsDeep(
          { a: 'b', c: 1, d: [2, { e: 'f' }] },
          { a: 'b', c: 5, d: [2, { e: 'f' }] },
          { ignoreUndefined }
        )
      ).toBe(false);
      expect(
        equalsDeep(
          { a: 'b', c: 1, d: [2, { e: 'f' }] },
          { a: 'b', c: 1, d: [5, { e: 'f' }] },
          { ignoreUndefined }
        )
      ).toBe(false);
      expect(
        equalsDeep(
          { a: 'b', c: 1, d: [2, { e: 'f' }] },
          { a: 'b', c: 1, d: [2, { e: 'f' }, 3] },
          { ignoreUndefined }
        )
      ).toBe(false);
      expect(
        equalsDeep(
          { a: 'b', c: 1, d: [2, { e: 'f' }] },
          { a: 'b', c: 1, d: [2, { e: 'z' }] },
          { ignoreUndefined }
        )
      ).toBe(false);
      expect(
        equalsDeep(
          { a: 'b', c: 1, d: [2, { e: 'f' }] },
          { a: 'b', c: 1, d: [2, { e: 'f' }], g: 'h' },
          { ignoreUndefined }
        )
      ).toBe(false);
      expect(
        equalsDeep(
          { a: 'b', c: 1, d: [{ e: 'f' }, 2] },
          { a: 'b', c: 1, d: [2, { e: 'f' }] },
          { ignoreUndefined }
        )
      ).toBe(false);
      expect(
        equalsDeep(
          { a: 'b', c: 1, d: [2, { e: 'f' }] },
          { a: 'b', z: 1, d: [2, { e: 'f' }] },
          { ignoreUndefined }
        )
      ).toBe(false);

      expect(
        equalsDeep(
          { a: 'b', c: 1, d: [2, { e: 'f' }] },
          { a: 'b', c: 1, d: [2, { e: 'f' }] },
          { ignoreUndefined }
        )
      ).toBe(true);
    });
  });

  it.each`
    options                       | result
    ${undefined}                  | ${false}
    ${{ ignoreUndefined: false }} | ${false}
    ${{ ignoreUndefined: true }}  | ${true}
  `('returns $result for options set to $options', ({ options, result }) => {
    expect(equalsDeep({ a: undefined, b: 1 }, { b: 1 }, options)).toBe(result);
    expect(equalsDeep({ b: 1 }, { a: undefined, b: 1 }, options)).toBe(result);
    expect(equalsDeep({}, { a: undefined }, options)).toBe(result);

    // Corner case: The same number of fields.
    expect(
      equalsDeep({ a: undefined, b: 1 }, { b: 1, c: undefined }, options)
    ).toBe(result);

    // And the same thing, but in the deep.
    expect(
      equalsDeep(
        { a: { b: undefined, c: 1 } },
        { a: { c: 1, d: undefined } },
        options
      )
    ).toBe(result);

    // Crossing an array.
    expect(
      equalsDeep(
        { a: [{ b: undefined, c: 1 }] },
        { a: [{ c: 1, d: undefined }] },
        options
      )
    ).toBe(result);
  });

  // A "real-world" example.
  it('compares role options', () => {
    const makeOptions = () => ({
      cert_format: 'standard',
      create_db_user: false,
      create_desktop_user: false,
      desktop_clipboard: true,
      desktop_directory_sharing: true,
      enhanced_recording: ['command', 'network'],
      forward_agent: false,
      idp: {
        saml: {
          enabled: true,
        },
      },
      max_session_ttl: '30h0m0s',
      pin_source_ip: false,
      port_forwarding: true,
      record_session: {
        default: 'best_effort',
        desktop: true,
      },
      ssh_file_copy: true,
    });

    expect(equalsDeep(makeOptions(), makeOptions())).toBe(true);
    expect(
      equalsDeep(makeOptions(), {
        ...makeOptions(),
        idp: { saml: { enabled: false } },
      })
    ).toBe(false);
    expect(
      equalsDeep(makeOptions(), {
        ...makeOptions(),
        unknown_option: 'foo',
      })
    ).toBe(false);
  });
});
