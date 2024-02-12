/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import Logger, { NullService } from 'teleterm/logger';

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

    await workspacesService.restorePersistedState();
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
      },
    });
  });

  it('creates empty workspace if there is no persisted state for given clusterUri', async () => {
    const cluster = makeRootCluster();
    const { workspacesService, clusterDocument } = getTestSetup({
      cluster,
      persistedWorkspaces: {},
    });

    await workspacesService.restorePersistedState();
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

        return { closeDialog: () => {} };
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

        return { closeDialog: () => {} };
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
    clusterDocument = {
      kind: 'doc.cluster',
      title: 'Cluster Test',
      clusterUri: cluster?.uri,
      uri: '/docs/test-cluster-uri',
    };
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
