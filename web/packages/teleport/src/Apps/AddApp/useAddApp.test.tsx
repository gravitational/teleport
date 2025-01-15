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

import { renderHook, waitFor } from '@testing-library/react';

import { ContextProvider } from 'teleport/index';
import { userContext } from 'teleport/Main/fixtures';
import { ProxyRequiresUpgrade } from 'teleport/services/version/unsupported';
import TeleportContext from 'teleport/teleportContext';

import useAddApp from './useAddApp';

const ctx = new TeleportContext();

beforeEach(() => {
  ctx.storeUser.setState({ ...userContext });
  jest
    .spyOn(ctx.joinTokenService, 'fetchJoinToken')
    .mockResolvedValue(tokenResp);
});

afterEach(() => {
  jest.resetAllMocks();
});

test('create token without labels', async () => {
  jest
    .spyOn(ctx.joinTokenService, 'fetchJoinTokenV2')
    .mockResolvedValue(tokenResp);

  const wrapper = ({ children }) => (
    <ContextProvider ctx={ctx}>{children}</ContextProvider>
  );

  let { result } = renderHook(() => useAddApp(ctx), { wrapper });

  await waitFor(() => {
    expect(result.current.token).not.toBeUndefined();
  });

  expect(ctx.joinTokenService.fetchJoinTokenV2).toHaveBeenCalledTimes(1);
  expect(ctx.joinTokenService.fetchJoinToken).not.toHaveBeenCalled();
  expect(result.current.token).toEqual(tokenResp);
});

test('create token without labels with v1 fallback', async () => {
  jest
    .spyOn(ctx.joinTokenService, 'fetchJoinTokenV2')
    .mockRejectedValueOnce(new Error(ProxyRequiresUpgrade));

  const wrapper = ({ children }) => (
    <ContextProvider ctx={ctx}>{children}</ContextProvider>
  );

  let { result } = renderHook(() => useAddApp(ctx), { wrapper });

  await waitFor(() => {
    expect(result.current.token).not.toBeUndefined();
  });

  expect(ctx.joinTokenService.fetchJoinTokenV2).toHaveBeenCalledTimes(1);
  expect(ctx.joinTokenService.fetchJoinToken).toHaveBeenCalledTimes(1);
  expect(result.current.token).toEqual(tokenResp);
});

const tokenResp = {
  allow: undefined,
  bot_name: undefined,
  content: undefined,
  expiry: null,
  expiryText: '',
  gcp: undefined,
  id: undefined,
  isStatic: undefined,
  method: undefined,
  internalResourceId: 'abc',
  roles: ['Application'],
  safeName: undefined,
  suggestedLabels: [],
};
