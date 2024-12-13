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
  DefaultTab,
  ViewMode,
  LabelsViewMode,
  AvailableResourceMode,
} from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import Logger, { NullService } from 'teleterm/logger';
import { makeDocumentCluster } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';

import { ClustersService } from '../clusters';
import { StatePersistenceService } from '../statePersistence';
import { NotificationsService } from '../notifications';
import { ModalsService } from '../modals';

import { getEmptyPendingAccessRequest } from './accessRequestsService';
import {
  Workspace,
  WorkspacesService,
  WorkspacesState,
} from './workspacesService';
import { DocumentCluster, DocumentsService } from './documentsService';

import type * as tshd from 'teleterm/services/tshd/types';

beforeAll(() => {
  Logger.init(new NullService());
});

beforeEach(() => {
  jest.restoreAllMocks();
});

describe('restoring workspace', () => {
  it('restores the workspace if there is a persisted state for given clusterUri', () => {
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

    const persistedWorkspace = { [cluster.uri]: testWorkspace };

    const { workspacesService } = getTestSetup({
      cluster,
      persistedWorkspaces: persistedWorkspace,
    });

    expect(workspacesService.state.isInitialized).toEqual(false);

    workspacesService.restorePersistedState();

    expect(workspacesService.state.isInitialized).toEqual(true);
    expect(workspacesService.getWorkspaces()).toStrictEqual({
      [cluster.uri]: {
        accessRequests: {
          pending: {
            kind: 'resource',
            resources: new Map(),
          },
          isBarCollapsed: false,
        },
        localClusterUri: testWorkspace.localClusterUri,
        documents: [expect.objectContaining({ kind: 'doc.cluster' })],
        location: expect.any(String),
        documentsRestoredOrDiscarded: false,
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

    expect(workspacesService.state.isInitialized).toEqual(false);

    workspacesService.restorePersistedState();

    expect(workspacesService.state.isInitialized).toEqual(true);
    expect(workspacesService.getWorkspaces()).toStrictEqual({
      [cluster.uri]: {
        accessRequests: {
          isBarCollapsed: false,
          pending: {
            kind: 'resource',
            resources: new Map(),
          },
        },
        localClusterUri: cluster.uri,
        documents: [expect.objectContaining({ kind: 'doc.cluster' })],
        location: expect.any(String),
        documentsRestoredOrDiscarded: false,
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
    const { workspacesService, notificationsService } = getTestSetup({
      cluster: makeRootCluster({
        connected: false,
        loggedInUser: undefined,
        profileStatusError: 'no YubiKey device connected',
      }),
      persistedWorkspaces: {},
    });

    jest.spyOn(notificationsService, 'notifyError');

    const { isAtDesiredWorkspace } =
      await workspacesService.setActiveWorkspace('/clusters/foo');

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
    const testWorkspace: Workspace = {
      accessRequests: {
        isBarCollapsed: true,
        pending: getEmptyPendingAccessRequest(),
      },
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

  const clustersService: Partial<ClustersService> = {
    findCluster: jest.fn(() => cluster),
    getRootClusters: () => [cluster].filter(Boolean),
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
