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

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import {
  makeAccessRequest,
  makeLoggedInUser,
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
  const accessRequest = makeAccessRequest();
  const cluster = makeRootCluster({
    features: { advancedAccessWorkflows: true, isUsageBasedBilling: false },
    loggedInUser: makeLoggedInUser({ activeRequests: [accessRequest.id] }),
  });
  appContext.addRootCluster(cluster);
  jest.spyOn(appContext.clustersService, 'dropRoles');
  const refreshListener = jest.fn();
  appContext.tshd.getAccessRequest = () => {
    return new MockedUnaryCall({
      request: accessRequest,
    });
  };

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

test('assumed request are always visible, even if getAccessRequests no longer returns them', async () => {
  const requestA = makeAccessRequest({
    id: '67bea2b6-9390-43a5-af47-c391561dbfba',
    resources: [],
    roles: ['access-a'],
  });
  const requestB = makeAccessRequest({
    id: '34488748-0c91-463e-ac93-a5dd21eeec8a',
    resources: [],
    roles: ['access-b'],
  });
  const appContext = new MockAppContext();
  const cluster = makeRootCluster({
    features: { advancedAccessWorkflows: true, isUsageBasedBilling: false },
    loggedInUser: makeLoggedInUser({
      activeRequests: [requestB.id, 'request-with-details-not-available'],
    }),
  });
  appContext.addRootCluster(cluster);

  appContext.tshd.getAccessRequests = () => {
    return new MockedUnaryCall({ requests: [requestA] });
  };
  appContext.tshd.getAccessRequest = ({ accessRequestId }) => {
    if (accessRequestId !== requestB.id) {
      throw new Error(`Request ${accessRequestId} not found`);
    }
    return new MockedUnaryCall({ request: requestB });
  };

  render(
    <MockAppContextProvider appContext={appContext}>
      <ResourcesContextProvider>
        <MockWorkspaceContextProvider rootClusterUri={cluster.uri}>
          <AccessRequestsContextProvider rootClusterUri={cluster.uri}>
            <SelectorMenu />
          </AccessRequestsContextProvider>
        </MockWorkspaceContextProvider>
      </ResourcesContextProvider>
    </MockAppContextProvider>
  );

  const accessRequestsMenu = await screen.findByTitle('Access Requests');
  await userEvent.click(accessRequestsMenu);

  // Even though getAccessRequests only returned requestA, the UI always displays the assumed requests:
  // - requestB, which details exists in the useAssumedRequests cache
  // - request-with-details-not-available, which is shown using only its ID (details missing)
  expect(await screen.findByText('access-a')).toBeInTheDocument();
  expect(await screen.findByText('access-b')).toBeInTheDocument();
  expect(
    await screen.findByText('request-with-details-not-available')
  ).toBeInTheDocument();
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
