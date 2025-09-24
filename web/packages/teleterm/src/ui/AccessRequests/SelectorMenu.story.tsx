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

import { useEffect, useLayoutEffect, useRef } from 'react';

import { ResourceID } from 'gen-proto-ts/teleport/lib/teleterm/v1/access_request_pb';
import { wait, waitForever } from 'shared/utils/wait';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import {
  makeAccessRequest,
  makeLoggedInUser,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { ResourcesContextProvider } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';

import { AccessRequestsContextProvider } from './AccessRequestsContext';
import { SelectorMenu } from './SelectorMenu';

export default {
  title: 'Teleterm/AccessRequests/SelectorMenu',
};

const rootCluster = makeRootCluster({
  features: { advancedAccessWorkflows: true, isUsageBasedBilling: false },
});
const resourceIds: ResourceID[] = [
  {
    kind: 'db',
    name: 'postgres',
    clusterName: 'postgres',
    subResourceName: '',
  },
  {
    kind: 'app',
    name: 'figma',
    clusterName: 'postgres',
    subResourceName: '',
  },
  {
    kind: 'db',
    name: 'aurora',
    clusterName: 'postgres',
    subResourceName: '',
  },
  {
    kind: 'kube_cluster',
    name: 'cookie',
    clusterName: 'postgres',
    subResourceName: '',
  },
  {
    kind: 'node',
    name: 'ubuntu-24-04-very-long-name',
    clusterName: 'postgres',
    subResourceName: '',
  },
  {
    kind: 'app',
    name: 'grafana',
    clusterName: 'postgres',
    subResourceName: '',
  },
];
const smallAccessRequest = makeAccessRequest();
const mediumAccessRequest = makeAccessRequest({
  id: '11929070-6886-77eb-90aa-c7223dd735',
  resources: resourceIds.slice(0, 4).map(id => ({
    id,
    details: { friendlyName: '', hostname: '' },
  })),
});
const largeAccessRequest = makeAccessRequest({
  id: '11929070-6886-77eb-90aa-c7223dd735',
  resources: resourceIds.map(id => ({
    id,
    details: { friendlyName: '', hostname: '' },
  })),
});

export function RequestAvailable() {
  const appContext = new MockAppContext();
  appContext.tshd.getAccessRequests = () =>
    new MockedUnaryCall({
      totalCount: 0,
      startKey: '',
      requests: [smallAccessRequest],
    });

  return <ShowState appContext={appContext} />;
}

export function NoRequestsAvailable() {
  const appContext = new MockAppContext();
  appContext.tshd.getAccessRequests = async () => {
    await wait(1000);
    return new MockedUnaryCall({
      totalCount: 0,
      startKey: '',
      requests: [],
    });
  };

  return <ShowState appContext={appContext} />;
}

export function LoadingRequests() {
  const appContext = new MockAppContext();
  const controllerRef = useRef(new AbortController());
  useEffect(() => {
    return () => controllerRef.current.abort();
  }, []);
  appContext.tshd.getAccessRequests = () =>
    waitForever(controllerRef.current.signal);

  return <ShowState appContext={appContext} />;
}

export function LoadingRequestsError() {
  const appContext = new MockAppContext();
  appContext.tshd.getAccessRequests = () =>
    new MockedUnaryCall(
      null,
      new Error(
        'connection error: desc = "transport: Error while dialing: failed to dial: unable to establish proxy stream\\n\\trpc error: code = Unavailable desc'
      )
    );

  return <ShowState appContext={appContext} />;
}

export function ResourceRequestAlreadyAssumed() {
  const appContext = new MockAppContext();
  appContext.tshd.getAccessRequests = () =>
    new MockedUnaryCall({
      totalCount: 0,
      startKey: '',
      requests: [smallAccessRequest, mediumAccessRequest],
    });

  return (
    <ShowState
      appContext={appContext}
      activeRequests={[smallAccessRequest.id]}
    />
  );
}

export function RequestWithManyResources() {
  const appContext = new MockAppContext();
  appContext.tshd.getAccessRequests = () =>
    new MockedUnaryCall({
      totalCount: 0,
      startKey: '',
      requests: [largeAccessRequest],
    });

  return <ShowState appContext={appContext} />;
}

export function AssumedRequestWithNoDetails() {
  const appContext = new MockAppContext();
  appContext.tshd.getAccessRequests = () =>
    new MockedUnaryCall({
      totalCount: 0,
      startKey: '',
      requests: [],
    });

  return (
    <ShowState
      appContext={appContext}
      activeRequests={['34488748-0c91-463e-ac93-a5dd21eeec8a']}
    />
  );
}

function ShowState({
  activeRequests = [],
  appContext,
}: {
  activeRequests?: string[];
  appContext: MockAppContext;
}) {
  appContext.addRootCluster({
    ...rootCluster,
    loggedInUser: makeLoggedInUser({
      activeRequests: activeRequests,
    }),
  });
  appContext.clustersService.assumeRoles = async () => {
    await wait(1000);
    throw new Error(
      'connection error: desc = "transport: Error while dialing: failed to dial: unable to establish proxy stream\\n\\trpc error: code = Unavailable desc'
    );
  };
  appContext.clustersService.dropRoles = async () => {
    await wait(1000);
    throw new Error(
      'connection error: desc = "transport: Error while dialing: failed to dial: unable to establish proxy stream\\n\\trpc error: code = Unavailable desc'
    );
  };
  appContext.tshd.getAccessRequest = async input => {
    const allRequests = await appContext.tshd.getAccessRequests({
      clusterUri: rootCluster.uri,
    });
    const request = allRequests.response.requests.find(
      a => a.id === input.accessRequestId
    );
    return new MockedUnaryCall({
      request,
    });
  };

  useLayoutEffect(() => {
    (
      document.querySelector('#access-requests-menu') as HTMLButtonElement
    )?.click();
  }, []);

  return (
    <MockAppContextProvider appContext={appContext}>
      <ResourcesContextProvider>
        <MockWorkspaceContextProvider rootClusterUri={rootCluster.uri}>
          <AccessRequestsContextProvider rootClusterUri={rootCluster.uri}>
            <SelectorMenu />
          </AccessRequestsContextProvider>
        </MockWorkspaceContextProvider>
      </ResourcesContextProvider>
    </MockAppContextProvider>
  );
}
