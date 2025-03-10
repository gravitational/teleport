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

import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import { MockMainProcessClient } from 'teleterm/mainProcess/fixtures/mocks';
import { MockPtyServiceClient } from 'teleterm/services/pty/fixtures/mocks';
import {
  MockTshClient,
  MockVnetClient,
} from 'teleterm/services/tshd/fixtures/mocks';
import { RuntimeSettings } from 'teleterm/types';
import AppContext from 'teleterm/ui/appContext';
import { Document } from 'teleterm/ui/services/workspacesService';

export class MockAppContext extends AppContext {
  constructor(runtimeSettings?: Partial<RuntimeSettings>) {
    const mainProcessClient = new MockMainProcessClient(runtimeSettings);
    const tshdClient = new MockTshClient();
    const vnetClient = new MockVnetClient();
    const ptyServiceClient = new MockPtyServiceClient();

    super({
      mainProcessClient,
      tshClient: tshdClient,
      vnetClient,
      ptyServiceClient,
      setupTshdEventContextBridgeService: () => {},
      getPathForFile: () => '',
    });
  }

  addRootClusterWithDoc(
    cluster: Cluster,
    doc: Document[] | Document | undefined
  ) {
    this.clustersService.setState(draftState => {
      draftState.clusters.set(cluster.uri, cluster);
    });
    const docs = Array.isArray(doc) ? doc : [doc];
    this.workspacesService.addWorkspace(cluster.uri);
    this.workspacesService.setState(draftState => {
      draftState.rootClusterUri = cluster.uri;
      draftState.workspaces[cluster.uri].documents = docs.filter(Boolean);
      draftState.workspaces[cluster.uri].location = docs[0]?.uri;
    });
  }

  addRootCluster(cluster: Cluster) {
    this.addRootClusterWithDoc(cluster, undefined);
  }
}
