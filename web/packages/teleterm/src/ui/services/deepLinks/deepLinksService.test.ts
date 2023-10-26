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
