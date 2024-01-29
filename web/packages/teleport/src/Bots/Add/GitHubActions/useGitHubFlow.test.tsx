/**
 * Teleport
 * Copyright (C) 20024  Gravitational, Inc.
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

import { parseRepoAddress } from './useGitHubFlow';

describe('parseRepoAddress with valid inputs', () => {
  const testCases = [
    {
      url: 'https://github.com/gravitational/teleport',
      expected: {
        host: 'github.com',
        owner: 'gravitational',
        repository: 'teleport',
      },
    },
    {
      url: 'github.com/gravitational/teleport',
      expected: {
        host: 'github.com',
        owner: 'gravitational',
        repository: 'teleport',
      },
    },
    {
      url: 'https://www.example.com/company/project',
      expected: {
        host: 'www.example.com',
        owner: 'company',
        repository: 'project',
      },
    },
    {
      url: 'www.example.com/company/project',
      expected: {
        host: 'www.example.com',
        owner: 'company',
        repository: 'project',
      },
    },
  ];
  test.each(testCases)(
    'should return repo="$expected" for url=$url',
    ({ url, expected }) => {
      const result = parseRepoAddress(url);
      expect(result).toEqual(expected);
    }
  );
});

describe('parseRepoAddress throws with invalid inputs', () => {
  const testCases = [
    {
      url: 'https://github.com',
      expectedErr:
        'URL expected to be in the format https://<host>/<owner>/<repository>',
    },
    {
      url: 'https://github.com/owner',
      expectedErr:
        'URL expected to be in the format https://<host>/<owner>/<repository>',
    },
    {
      url: 'invalid URL',
      expectedErr: 'Must be a valid URL',
    },
  ];
  test.each(testCases)(
    'should throw with message="$expectedErr" for url=$url',
    ({ url, expectedErr }) => {
      expect(() => parseRepoAddress(url)).toThrow(expectedErr);
    }
  );
});
