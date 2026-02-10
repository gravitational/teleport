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
            redirect_uri: null,
          },
        },
      },
      {
        input:
          'teleport://cluster.example.com/authenticate_web_device?id=123&token=234&redirect_uri=http://cluster.example.com/web/users',
        expectedURL: {
          host: 'cluster.example.com',
          hostname: 'cluster.example.com',
          port: '',
          pathname: '/authenticate_web_device',
          username: '',
          searchParams: {
            id: '123',
            token: '234',
            redirect_uri: 'http://cluster.example.com/web/users',
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
    {
      proxyHost: 'cluster.example.com:1337',
      path: '/authenticate_web_device',
      username: 'alice.bobson@example.com',
      searchParams: {
        token: '123',
        id: '123',
        redirect_uri: 'http://cluster.example.com:1337/web/users',
      },
    },
    {
      proxyHost: 'cluster.example.com:1337',
      path: '/authenticate_web_device',
      username: 'alice.bobson@example.com',
      searchParams: {
        token: '123',
        id: '123',
        redirect_uri:
          'https://cluster.example.com:1337/web/cluster/enterprise-local/resources?sort=name%3Adesc&pinnedOnly=false&kinds=app',
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
    'teleport://alice@cluster.example.com/authenticate_web_device?id=123&token=234&redirect_uri=http%3A%2F%2Fcluster.example.com%2Fweb%2Fusers',
    'teleport://alice@cluster.example.com/authenticate_web_device?id=123&token=234&redirect_uri=http%3A%2F%2Fcluster.example.com%2Fweb%2Fusers',
    // redirect_uri which includes its own query params.
    'teleport://alice@cluster.example.com:3030/authenticate_web_device?id=423535c9-5da5-4b0e-a3cc-4c629ec09848&token=m83K7v2waTYCJJsFRHlKHDWHZ7CszFwBTj5NHmG_32Q&redirect_uri=https%3A%2F%2Fcluster.example.com%3A3030%2Fweb%2Fcluster%2Fenterprise-local%2Fresources%3Fsort%3Dname%253Adesc%26pinnedOnly%3Dfalse%26kinds%3Dapp',
    // Triple-nested redirect_uri: it points to an app launch URL of the dumper app which is
    // supposed to redirect to a URL which in itself has a custom_url query param with some kind of
    // a URL with query params. You can imagine that originally the user wanted to visit this URL:
    // https://dumper.cluster.example.com:3030/hello?custom_url=https%3A%2F%2Fcluster.example.com%3A3030%2Fweb%2Fcluster%2Fenterprise-local%2Fresources%3Fsort%3Dname%253Adesc%26pinnedOnly%3Dfalse%26kinds%3Dapp
    'teleport://alice@cluster.example.com:3030/authenticate_web_device?id=e8fce168-3cb1-4731-90d6-a59b2aeb343e&token=5-8lLKZ0VPU9_dqkx2OhAXLwGMth005QlPtVWHfnvXU&redirect_uri=https%3A%2F%2Fcluster.example.com%3A3030%2Fweb%2Flaunch%2Fdumper.cluster.example.com%3Fpath%3D%252Fhello%26query%3Dcustom_url%253Dhttps%25253A%25252F%25252Fcluster.example.com%25253A3030%25252Fweb%25252Fcluster%25252Fenterprise-local%25252Fresources%25253Fsort%25253Dname%2525253Adesc%252526pinnedOnly%25253Dfalse%252526kinds%25253Dapp',
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
