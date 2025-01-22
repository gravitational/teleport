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

import {
  AvailableResourceMode,
  DefaultTab,
  LabelsViewMode,
  ViewMode,
} from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';

import Logger, { NullService } from 'teleterm/logger';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import type * as tshd from 'teleterm/services/tshd/types';
import { makeDocumentCluster } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';

import { ClustersService } from '../clusters';
import { ModalsService } from '../modals';
import { NotificationsService } from '../notifications';
import {
  PersistedWorkspace,
  StatePersistenceService,
  WorkspacesPersistedState,
} from '../statePersistence';
import { getEmptyPendingAccessRequest } from './accessRequestsService';
import { DocumentCluster, DocumentsService } from './documentsService';
import { WorkspacesService, WorkspacesState } from './workspacesService';

beforeAll(() => {
  Logger.init(new NullService());
});

beforeEach(() => {
  jest.restoreAllMocks();
});

describe('restoring workspace', () => {
  it('restores the workspace if there is a persisted state for given clusterUri', () => {
    const cluster = makeRootCluster();
    const testWorkspace: PersistedWorkspace = {
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

    const persistedWorkspace = { [cluster.uri]: testWorkspace };

    const { workspacesService } = getTestSetup({
      cluster,
      persistedWorkspaces: persistedWorkspace,
    });

    workspacesService.restorePersistedState();

    expect(workspacesService.getWorkspaces()).toStrictEqual({
      [cluster.uri]: {
        accessRequests: {
          pending: {
            kind: 'resource',
            resources: new Map(),
          },
          isBarCollapsed: false,
        },
        color: 'purple',
        localClusterUri: testWorkspace.localClusterUri,
        documents: [expect.objectContaining({ kind: 'doc.cluster' })],
        location: expect.any(String),
        hasDocumentsToReopen: true,
        connectMyComputer: undefined,
        unifiedResourcePreferences: {
          defaultTab: DefaultTab.ALL,
          viewMode: ViewMode.CARD,
          labelsViewMode: LabelsViewMode.COLLAPSED,
          availableResourceMode: AvailableResourceMode.NONE,
        },
      },
    });
    expect(workspacesService.getRestoredState().workspaces).toStrictEqual(
      persistedWorkspace
    );
  });

  it('creates empty workspace if there is no persisted state for given clusterUri', () => {
    const cluster = makeRootCluster();
    const { workspacesService } = getTestSetup({
      cluster,
      persistedWorkspaces: {},
    });

    workspacesService.restorePersistedState();

    expect(workspacesService.getWorkspaces()).toStrictEqual({
      [cluster.uri]: {
        accessRequests: {
          isBarCollapsed: false,
          pending: {
            kind: 'resource',
            resources: new Map(),
          },
        },
        color: 'purple',
        localClusterUri: cluster.uri,
        documents: [expect.objectContaining({ kind: 'doc.cluster' })],
        location: expect.any(String),
        hasDocumentsToReopen: false,
        connectMyComputer: undefined,
        unifiedResourcePreferences: {
          defaultTab: DefaultTab.ALL,
          viewMode: ViewMode.CARD,
          labelsViewMode: LabelsViewMode.COLLAPSED,
          availableResourceMode: AvailableResourceMode.NONE,
        },
      },
    });
    expect(workspacesService.getRestoredState().workspaces).toStrictEqual({});
  });

  it('restores profile color from state or assignes if empty', async () => {
    const clusterFoo = makeRootCluster({ uri: '/clusters/foo' });
    const workspaceFoo: PersistedWorkspace = {
      color: 'blue',
      localClusterUri: clusterFoo.uri,
      documents: [],
      location: undefined,
    };
    const clusterBar = makeRootCluster({ uri: '/clusters/bar' });
    const workspaceBar: PersistedWorkspace = {
      localClusterUri: clusterBar.uri,
      documents: [],
      location: undefined,
    };
    const clusterBaz = makeRootCluster({ uri: '/clusters/baz' });
    const workspaceBaz: PersistedWorkspace = {
      localClusterUri: clusterBaz.uri,
      documents: [],
      location: undefined,
    };

    const { workspacesService } = getTestSetup({
      cluster: [clusterFoo, clusterBar, clusterBaz],
      persistedWorkspaces: {
        [clusterFoo.uri]: workspaceFoo,
        [clusterBar.uri]: workspaceBar,
        [clusterBaz.uri]: workspaceBaz,
      },
    });

    workspacesService.restorePersistedState();

    expect(workspacesService.getWorkspace(clusterFoo.uri).color).toBe('blue'); // read from disk
    expect(workspacesService.getWorkspace(clusterBar.uri).color).toBe('purple'); // the first unused color
    expect(workspacesService.getWorkspace(clusterBaz.uri).color).toBe('green'); // the second unused color
  });
});

describe('state persistence', () => {
  it('doc.authorize_web_session is not stored to disk', () => {
    const cluster = makeRootCluster();
    const workspacesState: WorkspacesState = {
      rootClusterUri: cluster.uri,
      isInitialized: true,
      workspaces: {
        [cluster.uri]: {
          accessRequests: {
            isBarCollapsed: true,
            pending: getEmptyPendingAccessRequest(),
          },
          color: 'purple',
          localClusterUri: cluster.uri,
          documents: [
            {
              kind: 'doc.terminal_shell',
              uri: '/docs/terminal_shell_uri',
              title: '/Users/alice/Documents',
            },
            {
              kind: 'doc.authorize_web_session',
              uri: '/docs/authorize_web_session',
              rootClusterUri: cluster.uri,
              title: 'Authorize Web Session',
              webSessionRequest: {
                id: '',
                token: '',
                redirectUri: '',
                username: '',
              },
            },
          ],
          location: '/docs/authorize_web_session',
        },
      },
    };
    const { workspacesService, statePersistenceService } = getTestSetup({
      cluster,
      persistedWorkspaces: {},
    });

    workspacesService.setState(() => workspacesState);

    expect(statePersistenceService.saveWorkspacesState).toHaveBeenCalledTimes(
      1
    );
    expect(statePersistenceService.saveWorkspacesState).toHaveBeenCalledWith({
      rootClusterUri: cluster.uri,
      workspaces: {
        [cluster.uri]: expect.objectContaining({
          documents: [
            {
              kind: 'doc.terminal_shell',
              uri: '/docs/terminal_shell_uri',
              title: '/Users/alice/Documents',
            },
          ],
          location: '/docs/authorize_web_session',
        }),
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

  it('does not switch the workspace if the cluster has a profile status error', async () => {
    const rootCluster = makeRootCluster({
      connected: false,
      loggedInUser: undefined,
      profileStatusError: 'no YubiKey device connected',
    });
    const { workspacesService, notificationsService } = getTestSetup({
      cluster: rootCluster,
      persistedWorkspaces: {},
    });

    jest.spyOn(notificationsService, 'notifyError');

    const { isAtDesiredWorkspace } = await workspacesService.setActiveWorkspace(
      rootCluster.uri
    );

    expect(isAtDesiredWorkspace).toBe(false);
    expect(notificationsService.notifyError).toHaveBeenCalledWith(
      expect.objectContaining({
        title: 'Could not set cluster as active',
        description: 'no YubiKey device connected',
      })
    );
    expect(workspacesService.getRootClusterUri()).toBeUndefined();
  });

  it('sets location to first document if location points to non-existing document when reopening documents', async () => {
    const cluster = makeRootCluster();
    const testWorkspace: PersistedWorkspace = {
      localClusterUri: cluster.uri,
      documents: [
        {
          kind: 'doc.terminal_shell',
          uri: '/docs/terminal_shell_uri_1',
          title: '/Users/alice/Documents',
        },
        {
          kind: 'doc.terminal_shell',
          uri: '/docs/terminal_shell_uri_2',
          title: '/Users/alice/Documents',
        },
      ],
      location: '/docs/non-existing-doc',
    };

    const { workspacesService, modalsService } = getTestSetup({
      cluster,
      persistedWorkspaces: { [cluster.uri]: testWorkspace },
    });

    jest
      .spyOn(modalsService, 'openRegularDialog')
      .mockImplementation(dialog => {
        if (dialog.kind === 'documents-reopen') {
          dialog.onConfirm();
        } else {
          throw new Error(`Got unexpected dialog ${dialog.kind}`);
        }

        return {
          closeDialog: () => {},
        };
      });

    workspacesService.restorePersistedState();
    await workspacesService.setActiveWorkspace(cluster.uri);

    expect(workspacesService.getWorkspace(cluster.uri)).toStrictEqual(
      expect.objectContaining({
        documents: [
          {
            kind: 'doc.terminal_shell',
            uri: '/docs/terminal_shell_uri_1',
            title: '/Users/alice/Documents',
          },
          {
            kind: 'doc.terminal_shell',
            uri: '/docs/terminal_shell_uri_2',
            title: '/Users/alice/Documents',
          },
        ],
        location: '/docs/terminal_shell_uri_1',
      })
    );
  });

  it('ongoing setActive call is canceled when the method is called again', async () => {
    const clusterFoo = makeRootCluster({ uri: '/clusters/foo' });
    const clusterBar = makeRootCluster({ uri: '/clusters/bar' });
    const workspace1: PersistedWorkspace = {
      localClusterUri: clusterFoo.uri,
      documents: [
        {
          kind: 'doc.terminal_shell',
          uri: '/docs/terminal_shell_uri_1',
          title: '/Users/alice/Documents',
        },
      ],
      location: '/docs/non-existing-doc',
    };

    const { workspacesService } = getTestSetup({
      cluster: [clusterFoo, clusterBar],
      persistedWorkspaces: { [clusterFoo.uri]: workspace1 },
    });

    workspacesService.restorePersistedState();
    await Promise.all([
      // Activating the workspace foo will be stuck on restoring previous documents,
      // since we don't have any handler. This dialog will be canceled be a request
      // to set workspace bar (which doesn't have any documents).
      workspacesService.setActiveWorkspace(clusterFoo.uri),
      workspacesService.setActiveWorkspace(clusterBar.uri),
    ]);

    expect(workspacesService.getRootClusterUri()).toStrictEqual(clusterBar.uri);
  });
});

function getTestSetup(options: {
  cluster: tshd.Cluster | tshd.Cluster[] | undefined;
  persistedWorkspaces: WorkspacesPersistedState['workspaces'];
}) {
  const { cluster } = options;

  jest.mock('../modals');
  const ModalsServiceMock = ModalsService as jest.MockedClass<
    typeof ModalsService
  >;
  const modalsService = new ModalsServiceMock();

  jest.mock('../notifications');
  const NotificationsServiceMock = NotificationsService as jest.MockedClass<
    typeof NotificationsService
  >;
  const notificationsService = new NotificationsServiceMock();

  const statePersistenceService: Partial<StatePersistenceService> = {
    getWorkspacesState: () => ({
      workspaces: options.persistedWorkspaces,
    }),
    saveWorkspacesState: jest.fn(),
  };

  const normalizedClusters = (
    Array.isArray(cluster) ? cluster : [cluster]
  ).filter(Boolean);
  const clustersService: Partial<ClustersService> = {
    findCluster: jest.fn(clusterUri =>
      normalizedClusters.find(c => c.uri === clusterUri)
    ),
    getRootClusters: () => normalizedClusters,
    syncRootClustersAndCatchErrors: async () => {},
  };

  let clusterDocument: DocumentCluster;
  if (cluster) {
    clusterDocument = makeDocumentCluster();
  }

  const workspacesService = new WorkspacesService(
    modalsService,
    clustersService as ClustersService,
    notificationsService,
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

  return {
    workspacesService,
    modalsService,
    notificationsService,
    statePersistenceService,
  };
}
