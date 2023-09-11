/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import * as types from 'teleterm/ui/services/workspacesService';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

import { DocumentConnectMyComputerSetup } from './DocumentConnectMyComputerSetup';

export default {
  title: 'Teleterm/ConnectMyComputer/Setup',
};

export function Default() {
  const cluster = makeRootCluster();
  const doc: types.DocumentConnectMyComputerSetup = {
    kind: 'doc.connect_my_computer_setup',
    rootClusterUri: cluster.uri,
    title: 'Connect My Computer',
    uri: '/docs/123',
  };
  const appContext = new MockAppContext();
  appContext.clustersService.state.clusters.set(cluster.uri, cluster);
  appContext.workspacesService.setState(draftState => {
    draftState.rootClusterUri = cluster.uri;
    draftState.workspaces[cluster.uri] = {
      localClusterUri: cluster.uri,
      documents: [doc],
      location: doc.uri,
      accessRequests: undefined,
    };
  });

  return (
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={cluster.uri}>
        <DocumentConnectMyComputerSetup visible={true} doc={doc} />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
}
