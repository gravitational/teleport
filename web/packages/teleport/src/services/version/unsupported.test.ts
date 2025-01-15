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

import { integrationService } from '../integrations';
import { ProxyRequiresUpgrade, useV1Fallback } from './unsupported';

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
