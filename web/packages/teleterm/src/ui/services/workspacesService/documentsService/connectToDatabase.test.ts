/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import {
  makeDatabase,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { IAppContext } from 'teleterm/ui/types';

import { connectToDatabase } from './connectToDatabase';

describe('connectToDatabase', () => {
  test('creates gateway document with autoUserProvisioning when enabled', async () => {
    const appContext = new MockAppContext();
    setTestCluster(appContext);
    const database = makeDatabase();
    const autoUserProvisioning = {
      databaseRoles: ['reader'],
    };

    await connectToDatabase(
      appContext,
      {
        uri: database.uri,
        name: database.name,
        protocol: database.protocol,
        dbUser: 'alice',
        autoUserProvisioning,
      },
      { origin: 'resource_table' }
    );

    const documents = appContext.workspacesService
      .getActiveWorkspaceDocumentService()
      .getGatewayDocuments();
    expect(documents).toHaveLength(1);
    expect(documents[0]).toMatchObject({
      kind: 'doc.gateway',
      targetUri: database.uri,
      targetUser: 'alice',
      autoUserProvisioning,
    });
  });

  test('passes auto-provisioned username through unchanged for leaf cluster databases', async () => {
    const appContext = new MockAppContext();
    const rootCluster = makeRootCluster({
      uri: '/clusters/root' as const,
      name: 'root',
    });
    setTestCluster(appContext, rootCluster);

    const leafDatabase = makeDatabase({
      uri: '/clusters/root/leaves/leaf/dbs/postgres' as const,
    });

    await connectToDatabase(
      appContext,
      {
        uri: leafDatabase.uri,
        name: leafDatabase.name,
        protocol: 'postgres',
        dbUser: 'remote-alice-root',
        autoUserProvisioning: {
          databaseRoles: [],
        },
      },
      { origin: 'resource_table' }
    );

    const documents = appContext.workspacesService
      .getActiveWorkspaceDocumentService()
      .getGatewayDocuments();
    expect(documents[0].targetUser).toBe('remote-alice-root');
  });
});

function setTestCluster(
  appContext: IAppContext,
  cluster = makeRootCluster()
): void {
  appContext.workspacesService.setState(d => {
    d.rootClusterUri = cluster.uri;
  });
  appContext.clustersService.setState(d => {
    d.clusters.set(cluster.uri, cluster);
  });
}
