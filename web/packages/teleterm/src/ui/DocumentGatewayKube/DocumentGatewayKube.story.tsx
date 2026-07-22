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

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import * as types from 'teleterm/ui/services/workspacesService';

import { DocumentGatewayKube } from './DocumentGatewayKube';

export default {
  title: 'Teleterm/DocumentGatewayKube',
};

const rootCluster = makeRootCluster();
const offlineDocumentGateway: types.DocumentGatewayKube = {
  kind: 'doc.gateway_kube',
  rootClusterId: '',
  leafClusterId: '',
  targetUri: `${rootCluster.uri}/kubes/quux`,
  origin: 'resource_table',
  status: '',
  uri: '/docs/123',
  title: 'quux',
};

export function Offline() {
  const appContext = new MockAppContext();
  appContext.clustersService.createGateway = () =>
    Promise.reject(new Error('failed to create gateway'));
  appContext.addRootClusterWithDoc(rootCluster, offlineDocumentGateway);

  return (
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider>
        <DocumentGatewayKube doc={offlineDocumentGateway} visible={true} />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
}
