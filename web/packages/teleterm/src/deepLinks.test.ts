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

import { DeepURL, makeDeepLinkWithSafeInput } from 'shared/deepLinks';

import {
  DeepLinkParseResult,
  DeepLinkParseResultSuccess,
  parseDeepLink,
} from './deepLinks';

describe('parseDeepLink', () => {
  describe('valid input', () => {
    const tests: Array<{
      input: string;
      expectedURL: DeepURL;
    }> = [
      {
        input: 'teleport://cluster.example.com/connect_my_computer',
        expectedURL: {
          host: 'cluster.example.com',
          hostname: 'cluster.example.com',
          port: '',
          pathname: '/connect_my_computer',
          username: '',
          searchParams: {},
        },
      },
      {
        input:
          'teleport://cluster.example.com/authenticate_web_device?id=123&token=234',
        expectedURL: {
          host: 'cluster.example.com',
          hostname: 'cluster.example.com',
          port: '',
          pathname: '/authenticate_web_device',
          username: '',
          searchParams: {
            id: '123',
            token: '234',
          },
        },
      },
      {
        input: 'teleport://alice@cluster.example.com/connect_my_computer',
        expectedURL: {
          host: 'cluster.example.com',
          hostname: 'cluster.example.com',
          port: '',
          pathname: '/connect_my_computer',
          username: 'alice',
          searchParams: {},
        },
      },
      {
        input:
          'teleport://alice.bobson%40example.com@cluster.example.com:1337/connect_my_computer',
        expectedURL: {
          host: 'cluster.example.com:1337',
          hostname: 'cluster.example.com',
          port: '1337',
          pathname: '/connect_my_computer',
          username: 'alice.bobson@example.com',
          searchParams: {},
        },
      },
      // The example below is a bit contrived, usernames in URL should be percent-encoded. However,
      // Firefox and Safari will let you launch a link without percent-encoded username anyway, so
      // we just want to make sure that we correctly handle such cases.
      {
        input:
          'teleport://alice.bobson@example.com@cluster.example.com/connect_my_computer',
        expectedURL: {
          host: 'cluster.example.com',
          hostname: 'cluster.example.com',
          port: '',
          pathname: '/connect_my_computer',
          username: 'alice.bobson@example.com',
          searchParams: {},
        },
      },
    ];

    test.each(tests)('$input', ({ input, expectedURL }) => {
      const result = parseDeepLink(input);

      expect(result.status).toBe('success');
      expect(result.status === 'success' && result.url).toEqual(expectedURL);
    });
  });

  describe('invalid input', () => {
    const tests: Array<{ input: string; output: DeepLinkParseResult }> = [
      {
        input: 'teleport://hello\\foo@bar:baz',
        output: {
          status: 'error',
          reason: 'malformed-url',
          error: expect.any(TypeError),
        },
      },
      {
        input: 'teleport:///clusters/foo',
        output: {
          status: 'error',
          reason: 'unsupported-url',
        },
      },
      {
        input: 'teleport://cluster.example.com/foo',
        output: {
          status: 'error',
          reason: 'unsupported-url',
        },
      },
      {
        input: 'teleport:///foo/bar',
        output: {
          status: 'error',
          reason: 'unsupported-url',
        },
      },
      {
        input: 'foobar:///clusters/foo/connect_my_computer',
        output: {
          status: 'error',
          reason: 'unknown-protocol',
          protocol: 'foobar:',
        },
      },
      {
        input: 'teleport://cluster.example.com/authenticate_web_device',
        output: {
          error: new TypeError(
            'id and token must be included in the deep link for authenticating a web device'
          ),
          status: 'error',
          reason: 'malformed-url',
        },
      },
    ];

    test.each(tests)('$input', ({ input, output }) => {
      const result = parseDeepLink(input);
      expect(result).toEqual(output);
    });
  });
});

describe('makeDeepLinkWithSafeInput followed by parseDeepLink gives the same result', () => {
  const inputs: Array<Parameters<typeof makeDeepLinkWithSafeInput>[0]> = [
    {
      proxyHost: 'cluster.example.com',
      path: '/connect_my_computer',
      username: undefined,
      searchParams: {},
    },
    {
      proxyHost: 'cluster.example.com',
      path: '/connect_my_computer',
      username: 'alice',
      searchParams: {},
    },
    {
      proxyHost: 'cluster.example.com:1337',
      path: '/connect_my_computer',
      username: 'alice.bobson@example.com',
      searchParams: {},
    },
    {
      proxyHost: 'cluster.example.com:1337',
      path: '/authenticate_web_device',
      username: 'alice.bobson@example.com',
      searchParams: {
        token: '123',
        id: '123',
      },
    },
  ];

  test.each(inputs)('%j', input => {
    const deepLink = makeDeepLinkWithSafeInput(input);
    const parseResult = parseDeepLink(deepLink);
    expect(parseResult).toMatchObject({
      status: 'success',
      url: {
        host: input.proxyHost,
        pathname: input.path,
        username: input.username === undefined ? '' : input.username,
        searchParams: input.searchParams,
      },
    });
  });
});

describe('parseDeepLink followed by makeDeepLinkWithSafeInput gives the same result', () => {
  const inputs: string[] = [
    'teleport://cluster.example.com/connect_my_computer',
    'teleport://alice@cluster.example.com/connect_my_computer',
    'teleport://alice.bobson%40example.com@cluster.example.com:1337/connect_my_computer',
    'teleport://alice@cluster.example.com/authenticate_web_device?id=123&token=234',
  ];

  test.each(inputs)('%s', input => {
    const parseResult = parseDeepLink(input);
    expect(parseResult).toMatchObject({ status: 'success' });
    const { url } = parseResult as DeepLinkParseResultSuccess;
    const deepLink = makeDeepLinkWithSafeInput({
      proxyHost: url.host,
      path: url.pathname,
      username: url.username,
      searchParams: url.searchParams,
    });
    expect(deepLink).toEqual(input);
  });
});
