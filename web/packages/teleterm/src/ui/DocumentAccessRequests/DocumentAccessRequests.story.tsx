import React from 'react';

import { AccessRequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/access_request_pb';

import AppContextProvider from 'teleterm/ui/appContextProvider';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import { ResourcesContextProvider } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { DocumentAccessRequests } from 'teleterm/ui/DocumentAccessRequests/DocumentAccessRequests';
import * as types from 'teleterm/ui/services/workspacesService';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { getEmptyPendingAccessRequest } from 'teleterm/ui/services/workspacesService/accessRequestsService';
import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';

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

export function Loaded() {
  const appContext = new MockAppContext();
  appContext.tshd.getAccessRequests = () =>
    new MockedUnaryCall({
      totalCount: 0,
      startKey: '',
      requests: [mockedAccessRequest],
    });
  appContext.workspacesService.setState(draftState => {
    draftState.rootClusterUri = rootCluster.uri;
    draftState.workspaces[rootCluster.uri] = {
      localClusterUri: doc.clusterUri,
      documents: [doc],
      location: doc.uri,
      accessRequests: {
        pending: getEmptyPendingAccessRequest(),
        isBarCollapsed: true,
      },
    };
  });

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

export function ErrorLoading() {
  const appContext = new MockAppContext();
  appContext.tshd.getAccessRequests = () =>
    new MockedUnaryCall(null, new Error('network error'));
  appContext.workspacesService.setState(draftState => {
    draftState.rootClusterUri = rootCluster.uri;
    draftState.workspaces[rootCluster.uri] = {
      localClusterUri: doc.clusterUri,
      documents: [doc],
      location: doc.uri,
      accessRequests: {
        pending: getEmptyPendingAccessRequest(),
        isBarCollapsed: true,
      },
    };
  });
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(rootCluster.uri, rootCluster);
  });

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
