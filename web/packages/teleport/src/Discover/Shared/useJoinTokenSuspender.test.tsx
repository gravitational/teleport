/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport/index';
import {
  DiscoverEventResource,
  userEventService,
} from 'teleport/services/userEvent';
import { ProxyRequiresUpgrade } from 'teleport/services/version/unsupported';
import TeleportContext from 'teleport/teleportContext';

import { DiscoverContextState, DiscoverProvider } from '../useDiscover';
import { ResourceKind } from './ResourceKind';
import {
  clearCachedJoinTokenResult,
  useJoinTokenSuspender,
} from './useJoinTokenSuspender';

beforeEach(() => {
  jest
    .spyOn(userEventService, 'captureDiscoverEvent')
    .mockResolvedValue(undefined as never);
});

afterEach(() => {
  jest.resetAllMocks();
  clearCachedJoinTokenResult([ResourceKind.Server]);
});

test('create join token without labels', async () => {
  const ctx = new TeleportContext();

  jest
    .spyOn(ctx.joinTokenService, 'fetchJoinTokenV2')
    .mockResolvedValue(tokenResp);

  jest
    .spyOn(ctx.joinTokenService, 'fetchJoinToken')
    .mockResolvedValue(tokenResp);

  const wrapper = ({ children }) => (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <DiscoverProvider mockCtx={discoverCtx}>{children}</DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );

  let { result } = renderHook(
    () => useJoinTokenSuspender({ resourceKinds: [ResourceKind.Server] }),
    { wrapper }
  );

  await waitFor(() => {
    expect(result.current.joinToken).not.toBeNull();
  });

  expect(ctx.joinTokenService.fetchJoinTokenV2).toHaveBeenCalledTimes(1);
  expect(ctx.joinTokenService.fetchJoinToken).not.toHaveBeenCalled();
  expect(result.current.joinToken).toEqual(tokenResp);
});

test('create join token without labels with v1 fallback', async () => {
  const ctx = new TeleportContext();

  jest
    .spyOn(ctx.joinTokenService, 'fetchJoinTokenV2')
    .mockRejectedValueOnce(new Error(ProxyRequiresUpgrade));

  jest
    .spyOn(ctx.joinTokenService, 'fetchJoinToken')
    .mockResolvedValue(tokenResp);

  const wrapper = ({ children }) => (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <DiscoverProvider mockCtx={discoverCtx}>{children}</DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );

  let { result } = renderHook(
    () =>
      useJoinTokenSuspender({
        resourceKinds: [ResourceKind.Server],
        suggestedLabels: [],
      }),
    { wrapper }
  );

  await waitFor(() => {
    expect(result.current.joinToken).not.toBeNull();
  });

  expect(ctx.joinTokenService.fetchJoinTokenV2).toHaveBeenCalledTimes(1);
  expect(ctx.joinTokenService.fetchJoinToken).toHaveBeenCalledTimes(1);
  expect(result.current.joinToken).toEqual(tokenResp);
});

const discoverCtx: DiscoverContextState = {
  agentMeta: {},
  currentStep: 0,
  nextStep: () => null,
  prevStep: () => null,
  onSelectResource: () => null,
  resourceSpec: {
    name: 'Eks',
    kind: ResourceKind.Kubernetes,
    icon: 'eks',
    keywords: [],
    event: DiscoverEventResource.KubernetesEks,
  },
  exitFlow: () => null,
  viewConfig: null,
  indexedViews: [],
  setResourceSpec: () => null,
  updateAgentMeta: () => null,
  emitErrorEvent: () => null,
  emitEvent: () => null,
  eventState: null,
};

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
