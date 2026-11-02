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

import { AgentUsageEvidence } from 'gen-proto-ts/prehog/v1alpha/teleport_pb';

import { MfaChallengeResponse } from '../mfa';
import websession from '../websession';
import api, {
  AGENT_USAGE_HEADER,
  defaultRequestOptions,
  getAuthHeaders,
  isUserSessionInvalidError,
  MFA_HEADER,
} from './api';
import { ApiError } from './parseError';

jest.mock('../websession');
const mockedWebsession = jest.mocked(websession);

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

  test('no json in response', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue({ ok: true } as Response);
    const resp = await api.fetch('/something');
    expect(mockedFetch).toHaveBeenCalledTimes(1);

    expect(resp).toStrictEqual({ ok: true });
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

function setWebDriver(value: boolean | undefined) {
  Object.defineProperty(window.navigator, 'webdriver', {
    value,
    configurable: true,
  });
}

describe('api.fetch agent usage header', () => {
  let mockedFetch: jest.SpiedFunction<typeof fetch>;
  beforeEach(() => {
    mockedFetch = jest
      .spyOn(global, 'fetch')
      .mockResolvedValue({ ok: true } as Response);
  });

  afterEach(() => {
    setWebDriver(undefined);
    jest.restoreAllMocks();
  });

  test('sends the header when webdriver is true', async () => {
    setWebDriver(true);
    await api.fetch('/something');

    const [, actualRequestOptions] = mockedFetch.mock.calls[0];
    const headers = actualRequestOptions.headers as Record<string, string>;
    expect(headers['Content-Type']).toContain('application/json');
    const evidence = AgentUsageEvidence.fromBinary(
      Uint8Array.from(atob(headers[AGENT_USAGE_HEADER]), c => c.charCodeAt(0))
    );
    expect(evidence).toEqual(
      AgentUsageEvidence.create({ declaredWebDriver: true })
    );
  });

  test.each([false, undefined])(
    'sends no header when webdriver is %s',
    async value => {
      setWebDriver(value);
      await api.fetch('/something');

      const [, actualRequestOptions] = mockedFetch.mock.calls[0];
      expect(actualRequestOptions.headers).not.toHaveProperty(
        AGENT_USAGE_HEADER
      );
    }
  );
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

describe('isUserSessionInvalidError', () => {
  test('identifies a removed/expired user error', () => {
    const json = { error: { message: 'user expired or removed' } };
    expect(isUserSessionInvalidError(json)).toBe(true);
  });

  test('identifies a missing session role error', () => {
    const json = { error: { message: 'user session role not found' } };
    expect(isUserSessionInvalidError(json)).toBe(true);
  });

  test('returns false for an unrelated 403/404 error', () => {
    const json = { error: { message: 'role admin is not found' } };
    expect(isUserSessionInvalidError(json)).toBe(false);
  });

  // TODO(Joerger): DELETE IN v20.0.0 - covers the legacy wrapped message.
  test('falls back to the legacy wrapped message format', () => {
    const json = {
      error: 'role admin is not found',
      messages: ['user session role not found'],
    };
    expect(isUserSessionInvalidError(json)).toBe(true);
  });

  test('legacy fallback ignores irrelevant messages', () => {
    const json = {
      error: 'role admin is not found',
      messages: ['some_other_message'],
    };
    expect(isUserSessionInvalidError(json)).toBe(false);
  });

  test('handles a response with no error field at all', () => {
    expect(isUserSessionInvalidError({})).toBe(false);
  });
});

describe('handling of invalid session errors', () => {
  afterEach(() => {
    jest.resetAllMocks();
  });

  test.each([
    ['removed user', 'user expired or removed'],
    ['missing session role', 'user session role not found'],
  ])('sign out on %s error', async (_name, message) => {
    jest.spyOn(global, 'fetch').mockResolvedValue({
      json: async () => ({ error: { message } }),
      ok: false,
      status: 403,
    } as Response);

    await api.get('/foobar');
    expect(mockedWebsession.logoutWithoutSlo).toHaveBeenCalledWith({
      rememberLocation: false,
      withAccessChangedMessage: true,
    });
  });

  test("don't sign out on regular role not found error", async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue({
      json: async () => ({ error: { message: 'role foo is not found' } }),
      ok: false,
      status: 404,
    } as Response);

    await expect(api.get('/foobar')).rejects.toThrow(ApiError);
    expect(mockedWebsession.logoutWithoutSlo).not.toHaveBeenCalled();
  });
});
