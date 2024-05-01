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
import Logger, { NullService } from 'teleterm/logger';
import { makeDocumentCluster } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';

import { ClustersService } from '../clusters';
import { StatePersistenceService } from '../statePersistence';
import { NotificationsService } from '../notifications';
import { ModalsService } from '../modals';

import { getEmptyPendingAccessRequest } from './accessRequestsService';
import { Workspace, WorkspacesService } from './workspacesService';
import { DocumentCluster, DocumentsService } from './documentsService';

import type * as tshd from 'teleterm/services/tshd/types';

beforeAll(() => {
  Logger.init(new NullService());
});

beforeEach(() => {
  jest.restoreAllMocks();
});

describe('restoring workspace', () => {
  it('restores the workspace if there is a persisted state for given clusterUri', async () => {
    const cluster = makeRootCluster();
    const testWorkspace: Workspace = {
      accessRequests: {
        isBarCollapsed: true,
        pending: getEmptyPendingAccessRequest(),
      },
      localClusterUri: cluster.uri,
      documents: [
        {
          kind: 'doc.terminal_shell',
          uri: '/docs/some_uri',
          title: '/Users/alice/Documents',
        },
      ],
      location: '/docs/some_uri',
    };

    const { workspacesService, clusterDocument } = getTestSetup({
      cluster,
      persistedWorkspaces: { [cluster.uri]: testWorkspace },
    });

    expect(workspacesService.state.isInitialized).toEqual(false);

    await workspacesService.restorePersistedState();

    expect(workspacesService.state.isInitialized).toEqual(true);
    expect(workspacesService.getWorkspaces()).toStrictEqual({
      [cluster.uri]: {
        accessRequests: {
          pending: {
            app: {},
            db: {},
            kube_cluster: {},
            node: {},
            role: {},
            windows_desktop: {},
            user_group: {},
          },
          isBarCollapsed: false,
        },
        localClusterUri: testWorkspace.localClusterUri,
        documents: [clusterDocument],
        location: clusterDocument.uri,
        previous: {
          documents: testWorkspace.documents,
          location: testWorkspace.location,
        },
        connectMyComputer: undefined,
        unifiedResourcePreferences: undefined,
      },
    });
  });

  it('creates empty workspace if there is no persisted state for given clusterUri', async () => {
    const cluster = makeRootCluster();
    const { workspacesService, clusterDocument } = getTestSetup({
      cluster,
      persistedWorkspaces: {},
    });

    expect(workspacesService.state.isInitialized).toEqual(false);

    await workspacesService.restorePersistedState();

    expect(workspacesService.state.isInitialized).toEqual(true);
    expect(workspacesService.getWorkspaces()).toStrictEqual({
      [cluster.uri]: {
        accessRequests: {
          isBarCollapsed: false,
          pending: {
            app: {},
            db: {},
            kube_cluster: {},
            node: {},
            role: {},
            windows_desktop: {},
            user_group: {},
          },
        },
        localClusterUri: cluster.uri,
        documents: [clusterDocument],
        location: clusterDocument.uri,
        previous: undefined,
        connectMyComputer: undefined,
        unifiedResourcePreferences: undefined,
      },
    });
  });
});

describe('setActiveWorkspace', () => {
  it('switches the workspace for a cluster that is not connected', async () => {
    const cluster = makeRootCluster({
      connected: false,
    });
    const { workspacesService, modalsService } = getTestSetup({
      cluster,
      persistedWorkspaces: {},
    });

    // Resolve the modal immediately.
    jest
      .spyOn(modalsService, 'openRegularDialog')
      .mockImplementation(dialog => {
        if (dialog.kind === 'cluster-connect') {
          dialog.onSuccess(cluster.uri);
        } else {
          throw new Error(`Got unexpected dialog ${dialog.kind}`);
        }

        return {
          closeDialog: () => {},
        };
      });

    const { isAtDesiredWorkspace } = await workspacesService.setActiveWorkspace(
      cluster.uri
    );

    expect(isAtDesiredWorkspace).toBe(true);
    expect(workspacesService.getRootClusterUri()).toEqual(cluster.uri);
  });

  it('does not switch the workspace if the cluster is not in the state', async () => {
    const { workspacesService } = getTestSetup({
      cluster: undefined,
      persistedWorkspaces: {},
    });

    const { isAtDesiredWorkspace } =
      await workspacesService.setActiveWorkspace('/clusters/foo');

    expect(isAtDesiredWorkspace).toBe(false);
    expect(workspacesService.getRootClusterUri()).toBeUndefined();
  });

  it('does not switch the workspace if the login modal gets closed', async () => {
    const cluster = makeRootCluster({
      connected: false,
    });
    const { workspacesService, modalsService } = getTestSetup({
      cluster,
      persistedWorkspaces: {},
    });

    // Cancel the modal immediately.
    jest
      .spyOn(modalsService, 'openRegularDialog')
      .mockImplementation(dialog => {
        if (dialog.kind === 'cluster-connect') {
          dialog.onCancel();
        } else {
          throw new Error(`Got unexpected dialog ${dialog.kind}`);
        }

        return {
          closeDialog: () => {},
        };
      });

    const { isAtDesiredWorkspace } = await workspacesService.setActiveWorkspace(
      cluster.uri
    );

    expect(isAtDesiredWorkspace).toBe(false);
    expect(workspacesService.getRootClusterUri()).toBeUndefined();
  });
});

function getTestSetup(options: {
  cluster: tshd.Cluster | undefined; // assumes that only one cluster can be added
  persistedWorkspaces: Record<string, Workspace>;
}) {
  const { cluster } = options;

  jest.mock('../modals');
  const ModalsServiceMock = ModalsService as jest.MockedClass<
    typeof ModalsService
  >;
  const modalsService = new ModalsServiceMock();

  const statePersistenceService: Partial<StatePersistenceService> = {
    getWorkspacesState: () => ({
      workspaces: options.persistedWorkspaces,
    }),
    saveWorkspacesState: jest.fn(),
  };

  const clustersService: Partial<ClustersService> = {
    findCluster: jest.fn(() => cluster),
    getRootClusters: () => [cluster].filter(Boolean),
  };

  let clusterDocument: DocumentCluster;
  if (cluster) {
    clusterDocument = makeDocumentCluster();
  }

  const workspacesService = new WorkspacesService(
    modalsService,
    clustersService as ClustersService,
    new NotificationsService(),
    statePersistenceService as StatePersistenceService
  );

  workspacesService.getWorkspaceDocumentService = () =>
    ({
      createClusterDocument() {
        if (!clusterDocument) {
          throw new Error('getTestSetup received no cluster');
        }
        return clusterDocument;
      },
    }) as Partial<DocumentsService> as DocumentsService;

  return { workspacesService, clusterDocument, modalsService };
}
