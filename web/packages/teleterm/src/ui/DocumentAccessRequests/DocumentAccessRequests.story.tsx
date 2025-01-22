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

import { AccessRequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/access_request_pb';
import { ShowResources } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import AppContextProvider from 'teleterm/ui/appContextProvider';
import { DocumentAccessRequests } from 'teleterm/ui/DocumentAccessRequests/DocumentAccessRequests';
import { ResourcesContextProvider } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import * as types from 'teleterm/ui/services/workspacesService';

const rootCluster = makeRootCluster();

export default {
  title: 'Teleterm/DocumentAccessRequests',
};

const doc: types.DocumentAccessRequests = {
  kind: 'doc.access_requests',
  uri: '/docs/123',
  state: 'browsing',
  clusterUri: rootCluster.uri,
  requestId: '',
  title: 'Access Requests',
};

const mockedAccessRequest: AccessRequest = {
  id: '01929070-6886-77eb-90aa-c7223dd73f67',
  state: 'PENDING',
  resolveReason: '',
  requestReason: '',
  user: 'requester',
  roles: ['access', 'searcheable-resources'],
  reviews: [],
  suggestedReviewers: ['admin', 'reviewer'],
  thresholdNames: ['default'],
  resourceIds: [
    {
      kind: 'kube_cluster',
      name: 'minikube',
      clusterName: 'main',
      subResourceName: '',
    },
  ],
  resources: [
    {
      id: {
        kind: 'kube_cluster',
        name: 'minikube',
        clusterName: 'main',
        subResourceName: '',
      },
      details: { hostname: '', friendlyName: '' },
    },
  ],
  promotedAccessListTitle: '',
  created: { seconds: 1729000138n, nanos: 886521000 },
  expires: { seconds: 1729026573n, nanos: 0 },
  maxDuration: { seconds: 1729026573n, nanos: 0 },
  requestTtl: { seconds: 1729026573n, nanos: 0 },
  sessionTtl: { seconds: 1729026573n, nanos: 0 },
};

export function Browsing() {
  const appContext = new MockAppContext();
  appContext.tshd.getAccessRequests = () =>
    new MockedUnaryCall({
      totalCount: 0,
      startKey: '',
      requests: [mockedAccessRequest],
    });
  appContext.addRootClusterWithDoc(rootCluster, [doc]);

  return (
    <AppContextProvider value={appContext}>
      <MockWorkspaceContextProvider>
        <ResourcesContextProvider>
          <DocumentAccessRequests doc={doc} visible={true} />
        </ResourcesContextProvider>
      </MockWorkspaceContextProvider>
    </AppContextProvider>
  );
}

export function BrowsingError() {
  const appContext = new MockAppContext();
  appContext.tshd.getAccessRequests = () =>
    new MockedUnaryCall(null, new Error('network error'));
  appContext.addRootClusterWithDoc(rootCluster, [doc]);

  return (
    <AppContextProvider value={appContext}>
      <MockWorkspaceContextProvider>
        <ResourcesContextProvider>
          <DocumentAccessRequests doc={doc} visible={true} />
        </ResourcesContextProvider>
      </MockWorkspaceContextProvider>
    </AppContextProvider>
  );
}

export function CreatingWhenUnifiedResourcesShowOnlyAccessibleResources() {
  const docCreating: types.DocumentAccessRequests = {
    ...doc,
    state: 'creating',
  };
  const appContext = new MockAppContext();
  appContext.tshd.getAccessRequests = () =>
    new MockedUnaryCall({
      totalCount: 0,
      startKey: '',
      requests: [mockedAccessRequest],
    });
  appContext.addRootClusterWithDoc(
    {
      ...rootCluster,
      showResources: ShowResources.ACCESSIBLE_ONLY,
    },
    [doc]
  );

  return (
    <AppContextProvider value={appContext}>
      <MockWorkspaceContextProvider>
        <ResourcesContextProvider>
          <DocumentAccessRequests doc={docCreating} visible={true} />
        </ResourcesContextProvider>
      </MockWorkspaceContextProvider>
    </AppContextProvider>
  );
}

export function CreatingWhenUnifiedResourcesShowRequestableAndAccessibleResources() {
  const docCreating: types.DocumentAccessRequests = {
    ...doc,
    state: 'creating',
  };
  const appContext = new MockAppContext();
  appContext.tshd.getAccessRequests = () =>
    new MockedUnaryCall({
      totalCount: 0,
      startKey: '',
      requests: [mockedAccessRequest],
    });
  appContext.addRootClusterWithDoc(
    {
      ...rootCluster,
      showResources: ShowResources.REQUESTABLE,
    },
    [doc]
  );

  return (
    <AppContextProvider value={appContext}>
      <MockWorkspaceContextProvider>
        <ResourcesContextProvider>
          <DocumentAccessRequests doc={docCreating} visible={true} />
        </ResourcesContextProvider>
      </MockWorkspaceContextProvider>
    </AppContextProvider>
  );
}
