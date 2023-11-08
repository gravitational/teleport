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

import { Path, makeDeepLinkWithSafeInput } from 'shared/deepLinks';

import {
  DeepLinkParseResult,
  DeepLinkParseResultSuccess,
  parseDeepLink,
  DeepURL,
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
          reason: 'unsupported-uri',
        },
      },
      {
        input: 'teleport://cluster.example.com/foo',
        output: {
          status: 'error',
          reason: 'unsupported-uri',
        },
      },
      {
        input: 'teleport:///foo/bar',
        output: {
          status: 'error',
          reason: 'unsupported-uri',
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
      path: Path.ConnectMyComputer,
      username: undefined,
    },
    {
      proxyHost: 'cluster.example.com',
      path: Path.ConnectMyComputer,
      username: 'alice',
    },
    {
      proxyHost: 'cluster.example.com:1337',
      path: Path.ConnectMyComputer,
      username: 'alice.bobson@example.com',
    },
  ];

  test.each(inputs)('%j', input => {
    const deepLink = makeDeepLinkWithSafeInput(input);
    const parseResult = parseDeepLink(deepLink);
    expect(parseResult).toMatchObject({
      status: 'success',
      url: {
        host: input.proxyHost,
        pathname: '/' + input.path,
        username: input.username === undefined ? '' : input.username,
      },
    });
  });
});

describe('parseDeepLink followed by makeDeepLinkWithSafeInput gives the same result', () => {
  const inputs: string[] = [
    'teleport://cluster.example.com/connect_my_computer',
    'teleport://alice@cluster.example.com/connect_my_computer',
    'teleport://alice.bobson%40example.com@cluster.example.com:1337/connect_my_computer',
  ];

  test.each(inputs)('%s', input => {
    const parseResult = parseDeepLink(input);
    expect(parseResult).toMatchObject({ status: 'success' });
    const { url } = parseResult as DeepLinkParseResultSuccess;
    const deepLink = makeDeepLinkWithSafeInput({
      proxyHost: url.host,
      path: url.pathname.substring(1) as Path, // Remove the leading slash.
      username: url.username,
    });
    expect(deepLink).toEqual(input);
  });
});
