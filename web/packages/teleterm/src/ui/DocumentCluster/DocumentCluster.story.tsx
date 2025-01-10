/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { useEffect } from 'react';
import styled from 'styled-components';

import {
  leafClusterUri,
  makeAcl,
  makeApp,
  makeDatabase,
  makeKube,
  makeLoggedInUser,
  makeRootCluster,
  makeServer,
  rootClusterUri,
} from 'teleterm/services/tshd/testHelpers';
import * as tsh from 'teleterm/services/tshd/types';
import AppContextProvider from 'teleterm/ui/appContextProvider';
import { ConnectMyComputerContextProvider } from 'teleterm/ui/ConnectMyComputer';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import {
  ClustersServiceState,
  createClusterServiceState,
} from 'teleterm/ui/services/clusters';
import { ResourcesService } from 'teleterm/ui/services/resources';
import { getEmptyPendingAccessRequest } from 'teleterm/ui/services/workspacesService/accessRequestsService';
import { makeDocumentCluster } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';
import * as docTypes from 'teleterm/ui/services/workspacesService/documentsService/types';
import { ConnectionsContextProvider } from 'teleterm/ui/TopBar/Connections/connectionsContext';
import { routing } from 'teleterm/ui/uri';
import { VnetContextProvider } from 'teleterm/ui/Vnet';

import DocumentCluster from './DocumentCluster';
import { ResourcesContextProvider } from './resourcesContext';

export default {
  title: 'Teleterm/DocumentCluster',
};

const rootClusterDoc = makeDocumentCluster({
  clusterUri: rootClusterUri,
  uri: '/docs/123',
});

const leafClusterDoc = makeDocumentCluster({
  clusterUri: leafClusterUri,
  uri: '/docs/456',
});

export const OnlineLoadedResources = () => {
  const state = createClusterServiceState();
  state.clusters.set(
    rootClusterDoc.clusterUri,
    makeRootCluster({
      uri: rootClusterDoc.clusterUri,
    })
  );

  return renderState({
    state,
    doc: rootClusterDoc,
    listUnifiedResources: () =>
      Promise.resolve({
        resources: [
          {
            kind: 'server',
            resource: makeServer(),
            requiresRequest: false,
          },
          {
            kind: 'server',
            resource: makeServer({
              uri: `${rootClusterUri}/servers/1234`,
              hostname: 'bar',
              tunnel: true,
            }),
            requiresRequest: false,
          },
          {
            kind: 'database',
            resource: makeDatabase(),
            requiresRequest: false,
          },
          {
            kind: 'kube',
            resource: makeKube(),
            requiresRequest: false,
          },
          {
            kind: 'app',
            resource: { ...makeApp(), name: 'TCP app' },
            requiresRequest: false,
          },
          {
            kind: 'app',
            resource: {
              ...makeApp(),
              name: 'HTTP app',
              endpointUri: 'http://localhost:8080',
            },
            requiresRequest: false,
          },
          {
            kind: 'app',
            resource: {
              ...makeApp(),
              name: 'AWS console',
              endpointUri: 'https://localhost:8080',
              awsConsole: true,
              awsRoles: [
                {
                  arn: 'foo',
                  display: 'foo',
                  name: 'foo',
                  accountId: '123456789012',
                },
                {
                  arn: 'bar',
                  display: 'bar',
                  name: 'bar',
                  accountId: '123456789012',
                },
              ],
            },
            requiresRequest: true,
          },
          {
            kind: 'app',
            resource: {
              ...makeApp(),
              name: 'SAML app',
              desc: 'SAML Application',
              publicAddr: '',
              endpointUri: '',
              samlApp: true,
            },
            requiresRequest: true,
          },
        ],
        totalCount: 4,
        nextKey: '',
      }),
  });
};

export const OnlineEmptyResourcesAndCanAddResourcesAndConnectComputer = () => {
  const state = createClusterServiceState();
  state.clusters.set(
    rootClusterDoc.clusterUri,
    makeRootCluster({
      uri: rootClusterDoc.clusterUri,
      loggedInUser: makeLoggedInUser({
        userType: tsh.LoggedInUser_UserType.LOCAL,
        acl: makeAcl({
          tokens: {
            create: true,
            list: true,
            edit: true,
            delete: true,
            read: true,
            use: true,
          },
        }),
      }),
    })
  );

  return renderState({
    state,
    doc: rootClusterDoc,
    platform: 'darwin',
    listUnifiedResources: () =>
      Promise.resolve({
        resources: [],
        totalCount: 0,
        nextKey: '',
      }),
  });
};

export const OnlineEmptyResourcesAndCanAddResourcesButCannotConnectComputer =
  () => {
    const state = createClusterServiceState();
    state.clusters.set(
      rootClusterDoc.clusterUri,
      makeRootCluster({
        uri: rootClusterDoc.clusterUri,
        loggedInUser: makeLoggedInUser({
          userType: tsh.LoggedInUser_UserType.SSO,
          acl: makeAcl({
            tokens: {
              create: true,
              list: true,
              edit: true,
              delete: true,
              read: true,
              use: true,
            },
          }),
        }),
      })
    );

    return renderState({
      state,
      doc: rootClusterDoc,
      platform: 'win32',
      listUnifiedResources: () =>
        Promise.resolve({
          resources: [],
          totalCount: 0,
          nextKey: '',
        }),
    });
  };

export const OnlineEmptyResourcesAndCannotAddResources = () => {
  const state = createClusterServiceState();
  state.clusters.set(
    rootClusterDoc.clusterUri,
    makeRootCluster({
      uri: rootClusterDoc.clusterUri,
      loggedInUser: makeLoggedInUser({
        acl: makeAcl({
          tokens: {
            create: false,
            list: true,
            edit: true,
            delete: true,
            read: true,
            use: true,
          },
        }),
      }),
    })
  );

  return renderState({
    state,
    doc: rootClusterDoc,
    listUnifiedResources: () =>
      Promise.resolve({
        resources: [],
        totalCount: 0,
        nextKey: '',
      }),
  });
};

export const OnlineLoadingResources = () => {
  const state = createClusterServiceState();
  state.clusters.set(
    rootClusterDoc.clusterUri,
    makeRootCluster({
      uri: rootClusterDoc.clusterUri,
    })
  );

  let rejectPromise: (error: Error) => void;
  const promiseRejectedOnUnmount = new Promise<any>((resolve, reject) => {
    rejectPromise = reject;
  });

  useEffect(() => {
    return () => {
      rejectPromise(new Error('Aborted'));
    };
  }, [rejectPromise]);

  return renderState({
    state,
    doc: rootClusterDoc,
    listUnifiedResources: () => promiseRejectedOnUnmount,
  });
};

export const OnlineErrorLoadingResources = () => {
  const state = createClusterServiceState();
  state.clusters.set(
    rootClusterDoc.clusterUri,
    makeRootCluster({
      uri: rootClusterDoc.clusterUri,
    })
  );

  return renderState({
    state,
    doc: rootClusterDoc,
    listUnifiedResources: () =>
      Promise.reject(new Error('Whoops, something went wrong, sorry!')),
  });
};

export const Offline = () => {
  const state = createClusterServiceState();
  state.clusters.set(
    rootClusterDoc.clusterUri,
    makeRootCluster({
      connected: false,
      uri: rootClusterDoc.clusterUri,
    })
  );

  return renderState({ state, doc: rootClusterDoc });
};

export const Notfound = () => {
  const state = createClusterServiceState();
  state.clusters.set(
    rootClusterDoc.clusterUri,
    makeRootCluster({
      uri: rootClusterDoc.clusterUri,
    })
  );
  return renderState({ state, doc: leafClusterDoc });
};

function renderState({
  state,
  doc,
  listUnifiedResources,
  platform = 'darwin',
}: {
  state: ClustersServiceState;
  doc: docTypes.DocumentCluster;
  listUnifiedResources?: ResourcesService['listUnifiedResources'];
  platform?: NodeJS.Platform;
  userType?: tsh.LoggedInUser_UserType;
}) {
  const appContext = new MockAppContext({ platform });
  appContext.clustersService.state = state;

  const rootClusterUri = routing.ensureRootClusterUri(doc.clusterUri);
  appContext.workspacesService.setState(draftState => {
    draftState.rootClusterUri = rootClusterUri;
    draftState.workspaces[rootClusterUri] = {
      localClusterUri: doc.clusterUri,
      documents: [doc],
      location: doc.uri,
      accessRequests: {
        pending: getEmptyPendingAccessRequest(),
        isBarCollapsed: true,
      },
    };
  });

  appContext.resourcesService.listUnifiedResources = (params, abortSignal) =>
    listUnifiedResources
      ? listUnifiedResources(params, abortSignal)
      : Promise.reject('No fetchServersPromise passed');

  return (
    <AppContextProvider value={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <MockWorkspaceContextProvider>
            <ResourcesContextProvider>
              <ConnectMyComputerContextProvider rootClusterUri={rootClusterUri}>
                <Wrapper>
                  <DocumentCluster visible={true} doc={doc} />
                </Wrapper>
              </ConnectMyComputerContextProvider>
            </ResourcesContextProvider>
          </MockWorkspaceContextProvider>
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </AppContextProvider>
  );
}

const Wrapper = styled.div`
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
`;
