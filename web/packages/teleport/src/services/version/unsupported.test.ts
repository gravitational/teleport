/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { renderHook } from '@testing-library/react';

import { app } from 'teleport/Discover/AwsMangementConsole/fixtures';

import { ApiError } from '../api/parseError';
import { integrationService } from '../integrations';
import {
  ProxyRequiresUpgrade,
  useV1Fallback,
  withGenericUnsupportedError,
} from './unsupported';

afterEach(() => {
  jest.resetAllMocks();
});

test('with non upgrade proxy related error, re-throws error', async () => {
  jest.spyOn(integrationService, 'createAwsAppAccess');

  let { result } = renderHook(() => useV1Fallback());

  const err = new Error('some error');
  await expect(
    result.current.tryV1Fallback({
      kind: 'create-app-access',
      err,
      req: {},
      integrationName: 'foo',
    })
  ).rejects.toThrow(err);
  expect(integrationService.createAwsAppAccess).not.toHaveBeenCalled();
});

test('with upgrade proxy error, with labels, re-throws error', async () => {
  jest.spyOn(integrationService, 'createAwsAppAccess');

  let { result } = renderHook(() => useV1Fallback());

  const err = new Error(ProxyRequiresUpgrade);
  await expect(
    result.current.tryV1Fallback({
      kind: 'create-app-access',
      err,
      req: { labels: { env: 'dev' } },
      integrationName: 'foo',
    })
  ).rejects.toThrow(err);
  expect(integrationService.createAwsAppAccess).not.toHaveBeenCalled();
});

test('with upgrade proxy error, without labels, runs fallback', async () => {
  jest.spyOn(integrationService, 'createAwsAppAccess').mockResolvedValue(app);

  let { result } = renderHook(() => useV1Fallback());

  const err = new Error(ProxyRequiresUpgrade);
  const resp = await result.current.tryV1Fallback({
    kind: 'create-app-access',
    err,
    req: { labels: {} },
    integrationName: 'foo',
  });

  expect(resp).toEqual(app);
  expect(integrationService.createAwsAppAccess).toHaveBeenCalledTimes(1);
});

describe('withGenericUnsupportedError', () => {
  test('path not found error with proxy version throws custom error', async () => {
    const pathNotFoundError = new ApiError({
      message: '',
      response: { status: 404 } as Response,
      proxyVersion: {
        major: 1,
        minor: 2,
        patch: 3,
        preRelease: 'dev',
        string: 'v1.2.3-dev',
      },
    });

    expect(() =>
      withGenericUnsupportedError(pathNotFoundError, 'v2.0.0')
    ).toThrow('Your proxy (v1.2.3-dev) may be behind');

    expect(() =>
      withGenericUnsupportedError(pathNotFoundError, 'v2.0.0')
    ).toThrow('minimum required version (v2.0.0)');
  });

  test('legach path not found error throws custom error', async () => {
    const legacyPathNotFoundError = new ApiError({
      message: `404 - https://llama`,
      response: { status: 404, url: 'https://llama' } as Response,
    });

    expect(() =>
      withGenericUnsupportedError(legacyPathNotFoundError, 'v2.0.0')
    ).toThrow('Your proxy may be behind');
  });

  test('non path related 404 error rethrows same error', async () => {
    const resourceNotFoundError = new ApiError({
      message: `same error`,
      response: { status: 404 } as Response,
    });

    expect(() =>
      withGenericUnsupportedError(resourceNotFoundError, 'v2.0.0')
    ).toThrow('same error');
  });
});
