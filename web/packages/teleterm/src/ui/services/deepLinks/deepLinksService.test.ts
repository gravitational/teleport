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

import { DeepLinkParseResult } from 'teleterm/deepLinks';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { RuntimeSettings } from 'teleterm/types';

import { DeepLinksService } from './deepLinksService';

beforeEach(() => {
  jest.restoreAllMocks();
});

describe('parse errors', () => {
  const tests: Array<DeepLinkParseResult> = [
    {
      status: 'error',
      reason: 'malformed-url',
      error: new TypeError('whoops'),
    },
    { status: 'error', reason: 'unknown-protocol', protocol: 'foo:' },
    { status: 'error', reason: 'unsupported-uri' },
  ];

  test.each(tests)(
    '$reason causes a warning notification to be sent',
    async result => {
      const {
        clustersService,
        workspacesService,
        modalsService,
        notificationsService,
        runtimeSettings,
      } = getMocks();

      jest.spyOn(notificationsService, 'notifyWarning');
      jest.spyOn(modalsService, 'openRegularDialog');
      jest.spyOn(workspacesService, 'setActiveWorkspace');

      const deepLinksService = new DeepLinksService(
        runtimeSettings,
        clustersService,
        workspacesService,
        modalsService,
        notificationsService
      );
      await deepLinksService.launchDeepLink(result);

      expect(notificationsService.notifyWarning).toHaveBeenCalledTimes(1);
      expect(notificationsService.notifyWarning).toHaveBeenCalledWith({
        title: expect.stringContaining('Cannot open'),
        description: expect.any(String),
      });
      expect(modalsService.openRegularDialog).not.toHaveBeenCalled();
      expect(workspacesService.setActiveWorkspace).not.toHaveBeenCalled();
    }
  );
});

const cluster = makeRootCluster({
  uri: '/clusters/example.com',
  proxyHost: 'example.com:1234',
  name: 'example',
  connected: false,
});

const successResult: DeepLinkParseResult = {
  status: 'success',
  url: {
    host: cluster.proxyHost,
    hostname: 'example.com',
    port: '1234',
    pathname: '/connect_my_computer',
    username: 'alice',
  },
};

it('opens cluster connect dialog if the cluster is not added yet', async () => {
  const {
    clustersService,
    workspacesService,
    modalsService,
    notificationsService,
    runtimeSettings,
  } = getMocks();

  jest.spyOn(modalsService, 'openRegularDialog').mockImplementation(dialog => {
    if (dialog.kind !== 'cluster-connect') {
      throw new Error(`Got unexpected dialog ${dialog.kind}`);
    }

    // Mimick the cluster being added when going through the modal.
    clustersService.setState(draft => {
      draft.clusters.set(cluster.uri, { ...cluster, connected: true });
    });

    dialog.onSuccess(dialog.clusterUri);

    return { closeDialog: () => {} };
  });

  const deepLinksService = new DeepLinksService(
    runtimeSettings,
    clustersService,
    workspacesService,
    modalsService,
    notificationsService
  );

  await deepLinksService.launchDeepLink(successResult);

  expect(workspacesService.getRootClusterUri()).toEqual(cluster.uri);
  const documentsService = workspacesService.getWorkspaceDocumentService(
    cluster.uri
  );
  const activeDocument = documentsService.getActive();
  expect(activeDocument.kind).toBe('doc.connect_my_computer');
});

it('switches to the workspace if the cluster already exists', async () => {
  const {
    clustersService,
    workspacesService,
    modalsService,
    notificationsService,
    runtimeSettings,
  } = getMocks();

  clustersService.setState(draft => {
    draft.clusters.set(cluster.uri, { ...cluster, connected: true });
  });

  const deepLinksService = new DeepLinksService(
    runtimeSettings,
    clustersService,
    workspacesService,
    modalsService,
    notificationsService
  );

  await deepLinksService.launchDeepLink(successResult);

  expect(workspacesService.getRootClusterUri()).toEqual(cluster.uri);
  const documentsService = workspacesService.getWorkspaceDocumentService(
    cluster.uri
  );
  const activeDocument = documentsService.getActive();
  expect(activeDocument.kind).toBe('doc.connect_my_computer');
});

it('does not switch workspaces if the user does not log in to the cluster when adding it', async () => {
  const {
    clustersService,
    workspacesService,
    modalsService,
    notificationsService,
    runtimeSettings,
  } = getMocks();
  clustersService.setState(draft => {
    draft.clusters.set(cluster.uri, { ...cluster });
  });

  jest.spyOn(modalsService, 'openRegularDialog').mockImplementation(dialog => {
    if (dialog.kind !== 'cluster-connect') {
      throw new Error(`Got unexpected dialog ${dialog.kind}`);
    }

    // Mimick the cluster being closed without logging in.
    dialog.onCancel();

    return { closeDialog: () => {} };
  });

  const deepLinksService = new DeepLinksService(
    runtimeSettings,
    clustersService,
    workspacesService,
    modalsService,
    notificationsService
  );

  expect(workspacesService.getRootClusterUri()).toBeUndefined();

  await deepLinksService.launchDeepLink(successResult);

  expect(workspacesService.getRootClusterUri()).toBeUndefined();
});

it('sends a notification and does not switch workspaces if the user is on Windows', async () => {
  const {
    clustersService,
    workspacesService,
    modalsService,
    notificationsService,
    runtimeSettings,
  } = getMocks({ platform: 'win32' });

  jest.spyOn(notificationsService, 'notifyWarning');

  const deepLinksService = new DeepLinksService(
    runtimeSettings,
    clustersService,
    workspacesService,
    modalsService,
    notificationsService
  );

  expect(workspacesService.getRootClusterUri()).toBeUndefined();

  await deepLinksService.launchDeepLink(successResult);

  expect(workspacesService.getRootClusterUri()).toBeUndefined();
  expect(notificationsService.notifyWarning).toHaveBeenCalledTimes(1);
  expect(notificationsService.notifyWarning).toHaveBeenCalledWith(
    expect.stringContaining('not supported on Windows')
  );
});

function getMocks(partialRuntimeSettings?: Partial<RuntimeSettings>) {
  const {
    clustersService,
    workspacesService,
    modalsService,
    notificationsService,
    mainProcessClient,
  } = new MockAppContext(partialRuntimeSettings);
  const runtimeSettings = mainProcessClient.getRuntimeSettings();

  return {
    clustersService,
    workspacesService,
    modalsService,
    notificationsService,
    runtimeSettings,
  };
}
