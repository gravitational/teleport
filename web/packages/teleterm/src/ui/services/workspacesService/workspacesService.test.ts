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

import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';

import Logger, { NullService } from 'teleterm/logger';
import { createMockFileStorage } from 'teleterm/services/fileStorage/fixtures/mocks';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import type * as tshd from 'teleterm/services/tshd/types';
import {
  makeDocumentCluster,
  makeDocumentVnetDiagReport,
} from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';

import { ClustersService } from '../clusters';
import { ModalsService } from '../modals';
import { NotificationsService } from '../notifications';
import {
  StatePersistenceService,
  WorkspacesPersistedState,
} from '../statePersistence';
import {
  DocumentCluster,
  DocumentsService,
  DocumentVnetDiagReport,
} from './documentsService';
import { makePersistedWorkspace, makeWorkspace } from './testHelpers';
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
    const testWorkspace = makePersistedWorkspace({
      localClusterUri: cluster.uri,
      documents: [
        {
          kind: 'doc.terminal_shell',
          uri: '/docs/some_uri',
          title: '/Users/alice/Documents',
        },
      ],
      location: '/docs/some_uri',
    });

    const persistedWorkspace = { [cluster.uri]: testWorkspace };

    const { workspacesService } = getTestSetup({
      cluster,
      persistedWorkspaces: persistedWorkspace,
    });

    workspacesService.restorePersistedState();

    expect(workspacesService.getWorkspaces()).toStrictEqual({
      [cluster.uri]: makeWorkspace({
        proxyHost: cluster.proxyHost,
        localClusterUri: testWorkspace.localClusterUri,
        documents: [expect.objectContaining({ kind: 'doc.cluster' })],
        location: expect.any(String),
        hasDocumentsToReopen: true,
      }),
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
      [cluster.uri]: makeWorkspace({
        proxyHost: cluster.proxyHost,
        localClusterUri: cluster.uri,
        documents: [expect.objectContaining({ kind: 'doc.cluster' })],
        location: expect.any(String),
        hasDocumentsToReopen: false,
      }),
    });
    expect(workspacesService.getRestoredState().workspaces).toStrictEqual({});
  });

  it('keeps restored workspaces even when no matching cluster exists', () => {
    const cluster = makeRootCluster();
    const orphanClusterUri = '/clusters/orphan';
    const orphanWorkspace = makePersistedWorkspace({
      color: 'blue',
      localClusterUri: orphanClusterUri,
      documents: [
        {
          kind: 'doc.terminal_shell',
          uri: '/docs/some_orphan_doc',
          title: '/Users/alice/Downloads',
        },
      ],
      location: '/docs/some_orphan_doc',
    });

    const { workspacesService } = getTestSetup({
      cluster,
      persistedWorkspaces: {
        [orphanClusterUri]: orphanWorkspace,
      },
    });

    workspacesService.restorePersistedState();

    expect(workspacesService.getWorkspace(orphanClusterUri)).toBeDefined();
  });

  it('skips workspaces without a matching cluster or saved proxy host', () => {
    const orphanClusterUri = '/clusters/orphan';
    const orphanWorkspace = makePersistedWorkspace({
      // No stored proxy host.
      proxyHost: undefined,
      localClusterUri: orphanClusterUri,
    });

    const { workspacesService } = getTestSetup({
      // No matching cluster.
      cluster: undefined,
      persistedWorkspaces: {
        [orphanClusterUri]: orphanWorkspace,
      },
    });

    workspacesService.restorePersistedState();

    expect(workspacesService.getWorkspace(orphanClusterUri)).toBeUndefined();
  });

  it('keeps proxy host for restored workspaces without matching clusters', () => {
    const orphanClusterUri = '/clusters/orphan';
    const orphanWorkspace = makePersistedWorkspace({
      proxyHost: 'orphan.example.com:443',
      localClusterUri: orphanClusterUri,
    });

    const { workspacesService } = getTestSetup({
      cluster: undefined,
      persistedWorkspaces: {
        [orphanClusterUri]: orphanWorkspace,
      },
    });

    workspacesService.restorePersistedState();

    expect(workspacesService.getWorkspace(orphanClusterUri).proxyHost).toBe(
      orphanWorkspace.proxyHost
    );
  });

  it('restores workspace color from state or assigns if empty', async () => {
    const clusterFoo = makeRootCluster({ uri: '/clusters/foo' });
    const workspaceFoo = makePersistedWorkspace({
      color: 'blue',
      localClusterUri: clusterFoo.uri,
    });
    const clusterBar = makeRootCluster({ uri: '/clusters/bar' });
    const workspaceBar = makePersistedWorkspace({
      color: 'purple',
      localClusterUri: clusterBar.uri,
    });
    const clusterBaz = makeRootCluster({ uri: '/clusters/baz' });
    const clusterQux = makeRootCluster({ uri: '/clusters/qux' });
    const clusterWaldo = makeRootCluster({ uri: '/clusters/waldo' });
    const clusterFred = makeRootCluster({ uri: '/clusters/fred' });
    const clusterGrault = makeRootCluster({ uri: '/clusters/grault' });
    const clusterPlugh = makeRootCluster({ uri: '/clusters/plugh' });

    const { workspacesService } = getTestSetup({
      cluster: [
        clusterFoo,
        // The workspace for clusterBaz has no assigned color, but clusterBar's workspace does.
        // Return clusterBaz first to verify that it receives a new, unused color.
        clusterBaz,
        clusterBar,
        clusterQux,
        clusterWaldo,
        clusterFred,
        clusterGrault,
        clusterPlugh,
      ],
      persistedWorkspaces: {
        [clusterFoo.uri]: workspaceFoo,
        [clusterBar.uri]: workspaceBar,
      },
    });

    workspacesService.restorePersistedState();

    expect(workspacesService.getWorkspace(clusterFoo.uri).color).toBe('blue'); // read from disk
    expect(workspacesService.getWorkspace(clusterBar.uri).color).toBe('purple'); // read from disk
    expect(workspacesService.getWorkspace(clusterBaz.uri).color).toBe('green'); // the first unused color
    expect(workspacesService.getWorkspace(clusterQux.uri).color).toBe('yellow');
    expect(workspacesService.getWorkspace(clusterWaldo.uri).color).toBe('red');
    expect(workspacesService.getWorkspace(clusterFred.uri).color).toBe('cyan');
    expect(workspacesService.getWorkspace(clusterGrault.uri).color).toBe(
      'pink'
    );
    expect(workspacesService.getWorkspace(clusterPlugh.uri).color).toBe(
      'purple'
    ); // we have run out of colors, assign the default purple color
  });
});

describe('state persistence', () => {
  test('doc.authorize_web_session is not stored to disk', () => {
    const cluster = makeRootCluster();
    const workspacesState: WorkspacesState = {
      rootClusterUri: cluster.uri,
      isInitialized: true,
      workspaces: {
        [cluster.uri]: makeWorkspace({
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
        }),
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

  test('doc.vnet_diag_report has report.createdAt converted to string', async () => {
    const expectedCreatedAt = Timestamp.fromDate(new Date(2025, 0, 1, 12, 0));
    const cluster = makeRootCluster();
    const updatedWorkspacesState: WorkspacesState = {
      rootClusterUri: cluster.uri,
      isInitialized: true,
      workspaces: {
        [cluster.uri]: makeWorkspace({
          localClusterUri: cluster.uri,
          documents: [
            makeDocumentVnetDiagReport({
              report: {
                createdAt: expectedCreatedAt,
                checks: [],
              },
            }),
          ],
        }),
      },
    };

    const { workspacesService, statePersistenceService, modalsService } =
      getTestSetup({
        cluster,
        persistedWorkspaces: {},
      });

    // Confirm the reopen dialog immediately.
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

    // Verify that updating the state persists createdAt as string.
    workspacesService.setState(() => updatedWorkspacesState);
    const persistedDoc = statePersistenceService.getWorkspacesState()
      .workspaces[cluster.uri].documents[0] as DocumentVnetDiagReport;
    expect(persistedDoc).toBeTruthy();
    expect(persistedDoc.kind).toEqual('doc.vnet_diag_report');
    expect(persistedDoc.report.createdAt).toEqual(
      Timestamp.toJson(expectedCreatedAt)
    );

    // Verify that restoring persisted state converts createdAt back to Timestamp.
    workspacesService.restorePersistedState();
    await workspacesService.setActiveWorkspace(cluster.uri);
    const restoredDoc = workspacesService.state.workspaces[cluster.uri]
      .documents[0] as DocumentVnetDiagReport;
    expect(restoredDoc).toBeTruthy();
    expect(restoredDoc.kind).toEqual('doc.vnet_diag_report');
    expect(restoredDoc.report.createdAt).toEqual(expectedCreatedAt);
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
    workspacesService.addWorkspace(cluster);

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

  it('recreates a missing cluster from workspace proxy host before connecting', async () => {
    const clusterUri = '/clusters/foo.example.com';
    const proxyHost = 'foo.example.com:443';
    const { workspacesService, modalsService, clustersService } = getTestSetup({
      cluster: undefined,
      persistedWorkspaces: {
        [clusterUri]: makePersistedWorkspace({
          proxyHost,
          localClusterUri: clusterUri,
        }),
      },
    });
    workspacesService.restorePersistedState();
    expect(clustersService.findCluster(clusterUri)).toBeUndefined();

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

    const result = await workspacesService.setActiveWorkspace(clusterUri);

    expect(clustersService.addCluster).toHaveBeenCalledWith(proxyHost);
    expect(clustersService.findCluster(clusterUri)).toEqual(
      expect.objectContaining({
        uri: clusterUri,
        proxyHost,
        connected: false,
      })
    );
    expect(modalsService.openRegularDialog).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: 'cluster-connect',
        clusterUri,
      }),
      expect.any(AbortSignal)
    );
    expect(result.isAtDesiredWorkspace).toBe(false);
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
    workspacesService.addWorkspace(rootCluster);

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
    const testWorkspace = makePersistedWorkspace({
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
    });

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
    const workspace1 = makePersistedWorkspace({
      localClusterUri: clusterFoo.uri,
      documents: [
        {
          kind: 'doc.terminal_shell',
          uri: '/docs/terminal_shell_uri_1',
          title: '/Users/alice/Documents',
        },
      ],
      location: '/docs/non-existing-doc',
    });

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

  it('opens the documents-reopen dialog in the same tick as setting rootClusterUri', async () => {
    const cluster = makeRootCluster();
    const testWorkspace = makePersistedWorkspace({
      localClusterUri: cluster.uri,
      documents: [
        {
          kind: 'doc.terminal_shell',
          uri: '/docs/terminal_shell_uri',
          title: '/Users/alice/Documents',
        },
      ],
      location: '/docs/terminal_shell_uri',
    });

    const { workspacesService, modalsService } = getTestSetup({
      cluster,
      persistedWorkspaces: { [cluster.uri]: testWorkspace },
    });

    workspacesService.restorePersistedState();

    // Queue a microtask before starting activation. If an await is ever introduced between setting
    // rootClusterUri and opening the dialog, this microtask will execute before the dialog opens,
    // causing the assertion below to fail. This invariant matters because e2e tests rely on React
    // rendering rootClusterUri and the dialog in the same batch.
    let microtaskRan = false;
    queueMicrotask(() => {
      microtaskRan = true;
    });

    jest
      .spyOn(modalsService, 'openRegularDialog')
      .mockImplementation(dialog => {
        expect(dialog.kind).toEqual('documents-reopen');
        expect(microtaskRan).toBe(false);
        expect(workspacesService.getRootClusterUri()).toBe(cluster.uri);
        if (dialog.kind === 'documents-reopen') {
          dialog.onDiscard();
        }

        return {
          closeDialog: () => {},
        };
      });

    await workspacesService.setActiveWorkspace(cluster.uri);
    expect(modalsService.openRegularDialog).toHaveBeenCalledWith(
      expect.objectContaining({ kind: 'documents-reopen' }),
      expect.any(AbortSignal)
    );
  });
});

describe('clearWorkspace', () => {
  it('preserves proxy host and color for later workspace reuse', () => {
    const clusterFoo = makeRootCluster({ uri: '/clusters/foo' });
    const clusterBar = makeRootCluster({ uri: '/clusters/bar' });
    const { workspacesService } = getTestSetup({
      cluster: [clusterFoo, clusterBar],
      persistedWorkspaces: {},
    });

    workspacesService.restorePersistedState();
    workspacesService.changeWorkspaceColor(clusterFoo.uri, 'red');
    const previousProxyHost = workspacesService.getWorkspace(
      clusterFoo.uri
    ).proxyHost;
    const previousLocation = workspacesService.getWorkspace(
      clusterFoo.uri
    ).location;

    workspacesService.clearWorkspace(clusterFoo.uri);

    const workspace = workspacesService.getWorkspace(clusterFoo.uri);
    expect(workspace.color).toBe('red');
    expect(workspace.proxyHost).toBe(previousProxyHost);
    expect(workspace.documents).toHaveLength(1);
    expect(workspace.documents[0]).toEqual(
      expect.objectContaining({ kind: 'doc.cluster' })
    );
    expect(workspace.location).not.toEqual(previousLocation);
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

  const fileStorage = createMockFileStorage();
  fileStorage.put('state', {
    workspacesState: { workspaces: options.persistedWorkspaces },
  });

  const statePersistenceService = new StatePersistenceService(fileStorage);
  jest.spyOn(statePersistenceService, 'saveWorkspacesState');

  const normalizedClusters = (
    Array.isArray(cluster) ? cluster : [cluster]
  ).filter(Boolean);
  const clustersService: Partial<ClustersService> = {
    findCluster: jest.fn(clusterUri =>
      normalizedClusters.find(c => c.uri === clusterUri)
    ),
    getRootClusters: () => normalizedClusters,
    syncRootClustersAndCatchErrors: async () => {},
    addCluster: jest.fn(async proxy => {
      const cluster = makeRootCluster({
        uri: `/clusters/${proxy.replace(/:\d+$/, '')}`,
        proxyHost: proxy,
        connected: false,
        loggedInUser: undefined,
      });
      normalizedClusters.push(cluster);
      return cluster;
    }),
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
    clustersService,
    notificationsService,
    statePersistenceService,
  };
}
