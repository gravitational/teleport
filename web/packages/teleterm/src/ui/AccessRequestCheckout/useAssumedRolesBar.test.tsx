/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { act, renderHook } from '@testing-library/react';
import { useEffect } from 'react';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import {
  ResourcesContextProvider,
  useResourcesContext,
} from 'teleterm/ui/DocumentCluster/resourcesContext';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { RootClusterUri } from 'teleterm/ui/uri';

import { useAssumedRolesBar } from './useAssumedRolesBar';

test('dropping a request refreshes resources', async () => {
  const appContext = new MockAppContext();
  const cluster = makeRootCluster();
  appContext.addRootCluster(cluster);
  jest.spyOn(appContext.clustersService, 'dropRoles');
  const refreshListener = jest.fn();

  const wrapper = ({ children }) => (
    <MockAppContextProvider appContext={appContext}>
      <ResourcesContextProvider>
        <RequestRefreshSubscriber
          rootClusterUri={cluster.uri}
          onResourcesRefreshRequest={refreshListener}
        />
        {children}
      </ResourcesContextProvider>
    </MockAppContextProvider>
  );

  const { result } = renderHook(
    () =>
      useAssumedRolesBar({
        roles: [],
        id: 'mocked-request-id',
        expires: new Date(),
      }),
    { wrapper }
  );

  await act(() => result.current.dropRequest());
  expect(appContext.clustersService.dropRoles).toHaveBeenCalledTimes(1);
  expect(appContext.clustersService.dropRoles).toHaveBeenCalledWith(
    cluster.uri,
    ['mocked-request-id']
  );
  expect(refreshListener).toHaveBeenCalledTimes(1);
});

function RequestRefreshSubscriber(props: {
  rootClusterUri: RootClusterUri;
  onResourcesRefreshRequest: () => void;
}) {
  const { onResourcesRefreshRequest } = useResourcesContext(
    props.rootClusterUri
  );
  useEffect(() => {
    onResourcesRefreshRequest(props.onResourcesRefreshRequest);
  }, [onResourcesRefreshRequest, props.onResourcesRefreshRequest]);

  return null;
}
