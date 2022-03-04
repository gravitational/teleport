/**
 * Copyright 2020 Gravitational, Inc.
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
import styled from 'styled-components';
import AppContextProvider from 'teleterm/ui/appContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import {
  createClusterServiceState,
  ClustersServiceState,
} from 'teleterm/ui/services/clusters';
import DocumentCluster from './DocumentCluster';

export default {
  title: 'Teleterm/Cluster',
};

export const Online = () => {
  const state = createClusterServiceState();
  state.clusters.set('/clusters/localhost', {
    uri: '/clusters/localhost',
    leaf: false,
    name: 'localhost',
    connected: true,
  });

  return renderState(state);
};

export const Offline = () => {
  const state = createClusterServiceState();
  state.clusters.set('/clusters/localhost', {
    uri: '/clusters/localhost',
    leaf: false,
    name: 'localhost',
    connected: false,
  });

  return renderState(state);
};

export const Notfound = () => {
  const state = createClusterServiceState();
  return renderState(state);
};

function renderState(state: ClustersServiceState) {
  const appContext = new MockAppContext();
  appContext.workspacesService.getActiveWorkspaceDocumentService().update =
    () => null;
  appContext.clustersService.state = state;

  const doc = {
    kind: 'doc.cluster',
    clusterUri: '/clusters/localhost',
    uri: '123',
    title: 'sample',
  } as const;

  return (
    <AppContextProvider value={appContext}>
      <Wrapper>
        <DocumentCluster visible={true} doc={doc} />
      </Wrapper>
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
