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

import { processRedirectUri } from './processRedirectUri';

describe('processRedirectURI', () => {
  const tests: Array<{ name: string; input: string | null; expected: string }> =
    [
      {
        name: 'null input',
        input: null,
        expected: '/web',
      },
      {
        name: 'empty string',
        input: '',
        expected: '/web',
      },
      {
        name: 'valid internal URL',
        input: 'https://example.com/custom/path',
        expected: '/web/custom/path',
      },
      {
        name: 'handles URL encoded characters',
        input: 'https://example.com/path with spaces',
        expected: '/web/path%20with%20spaces',
      },
      {
        name: 'valid external URL',
        input: 'https://external.com/path',
        expected: '/web/path',
      },
      {
        name: 'URL with app access path',
        input: 'https://example.com/web/launch/myapp.example.com',
        expected: '/web/launch/myapp.example.com',
      },
      {
        name: 'invalid URL',
        input: '://invalid',
        expected: '/web',
      },
      {
        name: 'URL with empty path',
        input: 'https://example.com',
        expected: '/web',
      },
      {
        name: 'relative path',
        input: '/custom/path',
        expected: '/web/custom/path',
      },
      {
        name: 'path already starting with /web',
        input: '/web/existing/path',
        expected: '/web/existing/path',
      },
    ];

  test.each(tests)('$name', ({ input, expected }) => {
    const result = processRedirectUri(input);
    expect(result).toEqual(expected);
  });
});
