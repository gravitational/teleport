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

import { DeepLinkParseResult, parseDeepLink } from './deepLinks';
import { routing } from './ui/uri';

beforeEach(() => {
  jest.restoreAllMocks();
});

describe('parseDeepLink', () => {
  describe('valid input', () => {
    const tests: Array<string> = [
      'teleport:///clusters/foo/connect_my_computer',
      'teleport:///clusters/test.example.com/connect_my_computer?username=alice@example.com',
    ];

    test.each(tests)('%s', input => {
      jest.spyOn(routing, 'parseDeepLinkUri');
      const uri = input.replace('teleport://', '');

      const result = parseDeepLink(input);

      expect(result.status).toBe('success');
      expect(result.status === 'success' && result.parsedUri).not.toBeFalsy();
      expect(routing.parseDeepLinkUri).toHaveBeenCalledWith(uri);
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
      jest.spyOn(routing, 'parseDeepLinkUri').mockImplementation(() => null);

      const result = parseDeepLink(input);
      expect(result).toEqual(output);
    });
  });
});
