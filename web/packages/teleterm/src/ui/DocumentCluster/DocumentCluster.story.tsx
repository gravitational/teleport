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
import { routing } from 'teleterm/ui/uri';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

import * as docTypes from '../services/workspacesService/documentsService/types';

import DocumentCluster from './DocumentCluster';

export default {
  title: 'Teleterm/DocumentCluster',
};

const rootClusterDoc = {
  kind: 'doc.cluster' as const,
  clusterUri: '/clusters/localhost' as const,
  uri: '/docs/123' as const,
  title: 'sample',
};

const leafClusterDoc = {
  kind: 'doc.cluster' as const,
  clusterUri: '/clusters/localhost/leaves/foo' as const,
  uri: '/docs/456' as const,
  title: 'sample',
};

export const Online = () => {
  const state = createClusterServiceState();
  state.clusters.set(
    rootClusterDoc.clusterUri,
    makeRootCluster({
      uri: rootClusterDoc.clusterUri,
      name: 'localhost',
      proxyHost: 'localhost:3080',
    })
  );

  return renderState(state, rootClusterDoc);
};

export const Offline = () => {
  const state = createClusterServiceState();
  state.clusters.set(
    rootClusterDoc.clusterUri,
    makeRootCluster({
      uri: rootClusterDoc.clusterUri,
      name: 'localhost',
      proxyHost: 'localhost:3080',
      authClusterId: '73c4746b-d956-4f16-9848-4e3469f70762',
    })
  );

  return renderState(state, rootClusterDoc);
};

export const Notfound = () => {
  const state = createClusterServiceState();
  state.clusters.set(
    rootClusterDoc.clusterUri,
    makeRootCluster({
      uri: rootClusterDoc.clusterUri,
      name: 'localhost',
      proxyHost: 'localhost:3080',
    })
  );
  return renderState(state, leafClusterDoc);
};

function renderState(
  state: ClustersServiceState,
  doc: docTypes.DocumentCluster
) {
  const appContext = new MockAppContext();
  appContext.clustersService.state = state;

  appContext.workspacesService.setState(draftState => {
    const rootClusterUri = routing.ensureRootClusterUri(doc.clusterUri);
    draftState.rootClusterUri = rootClusterUri;
    draftState.workspaces[rootClusterUri] = {
      localClusterUri: doc.clusterUri,
      documents: [doc],
      location: doc.uri,
      accessRequests: undefined,
    };
  });

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
