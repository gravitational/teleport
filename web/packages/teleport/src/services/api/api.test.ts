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

import { MfaChallengeResponse } from '../mfa';
import api, {
  defaultRequestOptions,
  getAuthHeaders,
  isRoleNotFoundError,
  MFA_HEADER,
} from './api';

describe('api.fetch', () => {
  let mockedFetch: jest.SpiedFunction<typeof fetch>;
  beforeEach(() => {
    mockedFetch = jest
      .spyOn(global, 'fetch')
      .mockResolvedValue({ json: async () => ({}), ok: true } as Response); // we don't care about response
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  const mfaResp: MfaChallengeResponse = {
    webauthn_response: {
      id: 'some-id',
      type: 'some-type',
      extensions: {
        appid: false,
      },
      rawId: 'some-raw-id',
      response: {
        authenticatorData: 'authen-data',
        clientDataJSON: 'client-data-json',
        signature: 'signature',
        userHandle: 'user-handle',
      },
    },
  };

  const customOpts: RequestInit = {
    method: 'POST',
    // Override the default header from `defaultRequestOptions`.
    headers: {
      Accept: 'application/json',
    },
  };

  test('default (no optional params provided)', async () => {
    await api.fetch('/something');
    expect(mockedFetch).toHaveBeenCalledTimes(1);

    const firstCall = mockedFetch.mock.calls[0];
    const [actualUrl, actualRequestOptions] = firstCall;

    expect(actualUrl.toString().endsWith('/something')).toBe(true);
    expect(actualRequestOptions).toStrictEqual({
      ...defaultRequestOptions,
      headers: {
        ...defaultRequestOptions.headers,
        ...getAuthHeaders(),
      },
    });
  });

  test('with customOptions', async () => {
    await api.fetch('/something', customOpts);
    expect(mockedFetch).toHaveBeenCalledTimes(1);

    const firstCall = mockedFetch.mock.calls[0];
    const [, actualRequestOptions] = firstCall;

    expect(actualRequestOptions).toStrictEqual({
      ...defaultRequestOptions,
      ...customOpts,
      headers: {
        ...customOpts.headers,
        ...getAuthHeaders(),
      },
    });
  });

  test('with webauthnResponse', async () => {
    await api.fetch('/something', undefined, mfaResp);
    expect(mockedFetch).toHaveBeenCalledTimes(1);

    const firstCall = mockedFetch.mock.calls[0];
    const [, actualRequestOptions] = firstCall;

    expect(actualRequestOptions).toStrictEqual({
      ...defaultRequestOptions,
      headers: {
        ...defaultRequestOptions.headers,
        ...getAuthHeaders(),
        [MFA_HEADER]: JSON.stringify({
          ...mfaResp,
          webauthnAssertionResponse: mfaResp.webauthn_response,
        }),
      },
    });
  });

  test('with customOptions and webauthnResponse', async () => {
    await api.fetch('/something', customOpts, mfaResp);
    expect(mockedFetch).toHaveBeenCalledTimes(1);

    const firstCall = mockedFetch.mock.calls[0];
    const [, actualRequestOptions] = firstCall;

    expect(actualRequestOptions).toStrictEqual({
      ...defaultRequestOptions,
      ...customOpts,
      headers: {
        ...customOpts.headers,
        ...getAuthHeaders(),
        [MFA_HEADER]: JSON.stringify({
          ...mfaResp,
          webauthnAssertionResponse: mfaResp.webauthn_response,
        }),
      },
    });
  });

  const customContentType = {
    ...customOpts,
    headers: {
      Accept: 'application/json',
      'Content-Type': 'multipart/form-data',
    },
  };

  test('with customOptions including custom content-type', async () => {
    await api.fetch('/something', customContentType, null);
    expect(mockedFetch).toHaveBeenCalledTimes(1);

    const firstCall = mockedFetch.mock.calls[0];
    const [, actualRequestOptions] = firstCall;

    expect(actualRequestOptions).toStrictEqual({
      ...defaultRequestOptions,
      ...customOpts,
      headers: {
        ...customContentType.headers,
        ...getAuthHeaders(),
      },
    });
  });
});

// The code below should guard us from changes to api.fetchJsonWithMfaAuthnRetry which would cause it to lose type
// information, for example by returning `any`.

const fooService = {
  doSomething() {
    return api.fetchJsonWithMfaAuthnRetry('/foo', {}).then(makeFoo);
  },
};

const makeFoo = (): { foo: string } => {
  return { foo: 'lorem ipsum' };
};

// This is a bogus test to satisfy Jest. We don't even need to execute the code that's in the async
// function, we're interested only in the type system checking the code.
test('fetchJsonWithMfaAuthnRetry does not return any', () => {
  const bogusFunction = async () => {
    const result = await fooService.doSomething();
    // Reading foo is correct. We add a bogus expect to satisfy Jest.
    JSON.stringify(result.foo);

    // @ts-expect-error If there's no error here, it means that api.fetchJsonWithMfaAuthnRetry returns any, which it
    // shouldn't.
    JSON.stringify(result.bar);
  };
  bogusFunction.toString(); // Just to satisfy the linter

  expect(true).toBe(true);
});

test('isRoleNotFoundError correctly identifies role not found errors', () => {
  const errorMessage1 = 'role admin is not found';
  expect(isRoleNotFoundError(errorMessage1)).toBe(true);

  const errorMessage2 = '    role test-role is not found ';
  expect(isRoleNotFoundError(errorMessage2)).toBe(true);

  const errorMessage3 = 'failed to list access lists';
  expect(isRoleNotFoundError(errorMessage3)).toBe(false);
});
