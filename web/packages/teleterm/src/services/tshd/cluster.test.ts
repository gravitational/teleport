/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { proxyHostToBrowserProxyHost } from './cluster';

describe('proxyHostToBrowserProxyHost', () => {
  describe('valid inputs', () => {
    const tests: Array<{
      name: string;
      input: string;
      expectedOutput: string;
    }> = [
      {
        name: 'default port',
        input: 'cluster.example.com:443',
        expectedOutput: 'cluster.example.com',
      },
      {
        name: 'custom port',
        input: 'example.com:3090',
        expectedOutput: 'example.com:3090',
      },
      {
        name: 'top-level domain with default port',
        input: 'teleport-local:443',
        expectedOutput: 'teleport-local',
      },
      {
        name: 'top-level domain with custom port',
        input: 'teleport-local:3090',
        expectedOutput: 'teleport-local:3090',
      },
    ];

    test.each(tests)('$name ($input)', ({ input, expectedOutput }) => {
      expect(proxyHostToBrowserProxyHost(input)).toEqual(expectedOutput);
    });
  });

  describe('invalid inputs', () => {
    const tests: Array<{
      name: string;
      input: string;
    }> = [
      {
        name: 'proxyHost includes protocol',
        input: 'https://cluster.example.com:3090',
      },
      {
        name: 'whatwg-url parsing error',
        input: '<teleport>',
      },
    ];

    test.each(tests)('$name ($input)', ({ input }) => {
      expect(() => proxyHostToBrowserProxyHost(input)).toThrow(
        /invalid proxy host/i
      );
    });
  });
});
