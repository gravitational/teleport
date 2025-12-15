/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
import { act } from 'react';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { AppUpdaterContextProvider } from 'teleterm/ui/AppUpdater';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import { Props, useClusterLogin } from './useClusterLogin';

test('login into cluster and sync cluster', async () => {
  const appContext = new MockAppContext();
  const rootCluster = makeRootCluster();
  appContext.addRootCluster(rootCluster);

  jest.spyOn(appContext.tshd, 'login');

  const { result } = renderHook((props: Props) => useClusterLogin(props), {
    initialProps: {
      clusterUri: rootCluster.uri,
      prefill: { username: '' },
      onCancel: () => {},
      onSuccess: () => {},
    },
    wrapper: ({ children }) => (
      <MockAppContextProvider appContext={appContext}>
        <AppUpdaterContextProvider>{children}</AppUpdaterContextProvider>
      </MockAppContextProvider>
    ),
  });

  await waitFor(() => {
    expect(result.current.initAttempt.status).toEqual('success');
  });

  await act(async () => {
    result.current.onLoginWithLocal('user', 'password');
  });

  await waitFor(() => {
    expect(result.current.loginAttempt.status).toEqual('success');
  });

  expect(appContext.tshd.login).toHaveBeenCalledWith(
    {
      clusterUri: rootCluster.uri,
      params: {
        oneofKind: 'local',
        local: {
          user: 'user',
          password: 'password',
        },
      },
    },
    {
      abort: expect.objectContaining({ canBePassedThroughContextBridge: true }),
    }
  );
  const foundCluster = appContext.clustersService.findCluster(rootCluster.uri);
  expect(foundCluster).not.toBeUndefined();
  expect(foundCluster.connected).toBe(true);
});
