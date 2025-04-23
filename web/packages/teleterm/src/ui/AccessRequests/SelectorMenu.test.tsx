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

import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useEffect } from 'react';

import { render } from 'design/utils/testing';

import {
  makeAccessRequest,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { SelectorMenu } from 'teleterm/ui/AccessRequests/SelectorMenu';
import {
  ResourcesContextProvider,
  useResourcesContext,
} from 'teleterm/ui/DocumentCluster/resourcesContext';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import { RootClusterUri } from 'teleterm/ui/uri';

import { AccessRequestsContextProvider } from './AccessRequestsContext';

test('assuming or dropping a request refreshes resources', async () => {
  const appContext = new MockAppContext();
  const cluster = makeRootCluster({
    features: { advancedAccessWorkflows: true, isUsageBasedBilling: false },
  });
  appContext.addRootCluster(cluster);
  jest.spyOn(appContext.clustersService, 'dropRoles');
  const refreshListener = jest.fn();
  const accessRequest = makeAccessRequest();
  appContext.clustersService.setState(draftState => {
    draftState.clusters.get(cluster.uri).loggedInUser.assumedRequests = {
      [accessRequest.id]: accessRequest,
    };
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <ResourcesContextProvider>
        <MockWorkspaceContextProvider rootClusterUri={cluster.uri}>
          <AccessRequestsContextProvider rootClusterUri={cluster.uri}>
            <RequestRefreshSubscriber
              rootClusterUri={cluster.uri}
              onResourcesRefreshRequest={refreshListener}
            />
            <SelectorMenu />
          </AccessRequestsContextProvider>
        </MockWorkspaceContextProvider>
      </ResourcesContextProvider>
    </MockAppContextProvider>
  );

  const accessRequestsMenu = await screen.findByTitle('Access Requests');
  await userEvent.click(accessRequestsMenu);

  const item = await screen.findByText(accessRequest.resources.at(0).id.name);
  await userEvent.click(item);
  expect(appContext.clustersService.dropRoles).toHaveBeenCalledTimes(1);
  expect(appContext.clustersService.dropRoles).toHaveBeenCalledWith(
    cluster.uri,
    [accessRequest.id]
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
