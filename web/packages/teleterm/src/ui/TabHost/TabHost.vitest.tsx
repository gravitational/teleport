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

import { waitFor } from '@testing-library/react';
import { createRef } from 'react';
import { beforeAll, expect, test, vi } from 'vitest';

import { act, fireEvent, render, screen } from 'design/utils/testing';
import type { Gateway } from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';

import Logger, { NullService } from 'teleterm/logger';
import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';
import type { Shell } from 'teleterm/mainProcess/shell';
import { TabContextMenuOptions } from 'teleterm/mainProcess/types';
import {
  makeKubeGateway,
  makeRootCluster,
  rootClusterUri,
} from 'teleterm/services/tshd/testHelpers';
import { ResourcesContextProvider } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { Document } from 'teleterm/ui/services/workspacesService';
import {
  makeDocumentCluster,
  makeDocumentGatewayKube,
  makeDocumentPtySession,
} from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';
import { TabHost } from 'teleterm/ui/TabHost/TabHost';
import { routing } from 'teleterm/ui/uri';

function getMockDocuments(): Document[] {
  return [
    {
      kind: 'doc.blank',
      uri: '/docs/test_uri_1',
      title: 'Test 1',
    },
    {
      kind: 'doc.blank',
      uri: '/docs/test_uri_2',
      title: 'Test 2',
    },
  ];
}

async function getTestSetup({
  documents,
  availableShells,
  gateway,
}: {
  documents: Document[];
  availableShells?: Shell[];
  gateway?: Gateway;
}) {
  const runtimeSettings = makeRuntimeSettings();
  if (availableShells?.length) {
    runtimeSettings.availableShells = availableShells;
    runtimeSettings.defaultOsShellId = availableShells[0].id;
  }
  const appContext = new MockAppContext(runtimeSettings);
  vi.spyOn(appContext.mainProcessClient, 'openTabContextMenu');
  vi.spyOn(appContext.terminalsService, 'createPtyProcess');

  appContext.addRootClusterWithDoc(makeRootCluster(), documents);
  if (gateway) {
    appContext.clustersService.setState(draftState => {
      draftState.gateways.set(gateway.uri, gateway);
    });
  }

  const docsService =
    appContext.workspacesService.getActiveWorkspaceDocumentService();

  vi.spyOn(docsService, 'add');
  vi.spyOn(docsService, 'open');
  vi.spyOn(docsService, 'close');
  vi.spyOn(docsService, 'swapPosition');
  vi.spyOn(docsService, 'closeOthers');
  vi.spyOn(docsService, 'closeToRight');
  vi.spyOn(docsService, 'duplicatePtyAndActivate');
  vi.spyOn(docsService, 'reopenPtyInShell');

  render(
    <MockAppContextProvider appContext={appContext}>
      <ResourcesContextProvider>
        <TabHost
          ctx={appContext}
          topBarConnectMyComputerRef={createRef()}
          topBarAccessRequestRef={createRef()}
          desktopSessionControlsRef={createRef()}
        />
      </ResourcesContextProvider>
    </MockAppContextProvider>
  );

  // Mostly a bogus await just so that all useEffects in all of the mounted contexts have time to be
  // processed and not throw an error due to a state update outside of `act`.
  expect(await screen.findByTitle(/New Tab/)).toBeInTheDocument();

  return {
    docsService,
    mainProcessClient: appContext.mainProcessClient,
    terminalsService: appContext.terminalsService,
  };
}

beforeAll(() => {
  Logger.init(new NullService());
});

test('render documents', async () => {
  const { docsService } = await getTestSetup({
    documents: getMockDocuments(),
  });
  const documents = docsService.getDocuments();

  expect(screen.getByTitle(documents[0].title)).toBeInTheDocument();
  expect(screen.getByTitle(documents[1].title)).toBeInTheDocument();
});

test('open tab on click', async () => {
  const { docsService } = await getTestSetup({
    documents: [getMockDocuments()[0]],
  });
  const documents = docsService.getDocuments();
  const { open } = docsService;
  const $tabTitle = screen.getByTitle(documents[0].title);

  fireEvent.click($tabTitle);

  expect(open).toHaveBeenCalledWith(documents[0].uri);
});

test('open context menu', async () => {
  const { docsService, mainProcessClient } = await getTestSetup({
    documents: [getMockDocuments()[0]],
  });
  const { openTabContextMenu } = mainProcessClient;
  const { close, closeOthers, closeToRight, duplicatePtyAndActivate } =
    docsService;
  const documents = docsService.getDocuments();
  const document = documents[0];
  const spy = vi.spyOn(mainProcessClient, 'openTabContextMenu');

  const $tabTitle = screen.getByTitle(documents[0].title);

  fireEvent.contextMenu($tabTitle);
  expect(openTabContextMenu).toHaveBeenCalled();

  const options: TabContextMenuOptions = spy.mock.calls[0][0];
  expect(options.capabilities).toEqual({ canDuplicatePty: false });

  act(() => {
    options.onClose();
  });
  expect(close).toHaveBeenCalledWith(document.uri);

  act(() => {
    options.onCloseOthers();
  });
  expect(closeOthers).toHaveBeenCalledWith(document.uri);

  act(() => {
    options.onCloseToRight();
  });
  expect(closeToRight).toHaveBeenCalledWith(document.uri);

  act(() => {
    options.onDuplicatePty();
  });
  expect(duplicatePtyAndActivate).toHaveBeenCalledWith(document.uri);
});

const currentShell: Shell = {
  id: 'shell-1',
  binPath: '/bin/shell-1',
  binName: 'shell-1',
  friendlyName: 'shell-1',
};
const selectedShell: Shell = {
  id: 'shell-2',
  binPath: '/bin/shell-2',
  binName: 'shell-2',
  friendlyName: 'shell-2',
};
const availableShells = [currentShell, selectedShell];

test.each([
  {
    documentType: 'PTY session',
    expectedCanDuplicatePty: true,
    setupOptions: {
      documents: [makeDocumentPtySession({ shellId: currentShell.id })],
      availableShells,
    },
  },
  {
    documentType: 'Kube gateway',
    expectedCanDuplicatePty: false,
    setupOptions: {
      documents: [makeDocumentGatewayKube({ shellId: currentShell.id })],
      availableShells,
      gateway: makeKubeGateway(),
    },
  },
])(
  'change shell for $documentType document from context menu',
  async ({ expectedCanDuplicatePty, setupOptions }) => {
    const [document] = setupOptions.documents;
    const { terminalsService, mainProcessClient } =
      await getTestSetup(setupOptions);
    const openTabContextMenuSpy = vi.mocked(
      mainProcessClient.openTabContextMenu
    );
    const { createPtyProcess } = terminalsService;

    await waitFor(() => {
      expect(createPtyProcess).toHaveBeenCalledTimes(1);
    });
    expect(createPtyProcess).toHaveBeenLastCalledWith(
      expect.objectContaining({
        kind: 'pty.shell',
        shellId: currentShell.id,
      })
    );

    fireEvent.contextMenu(screen.getByTitle(document.title));

    expect(openTabContextMenuSpy).toHaveBeenCalledOnce();
    const options: TabContextMenuOptions =
      openTabContextMenuSpy.mock.calls[0][0];
    expect(options.capabilities).toEqual({
      canDuplicatePty: expectedCanDuplicatePty,
      shellSelector: {
        activeShellId: currentShell.id,
      },
    });

    act(() => {
      options.onReopenPtyInShell(selectedShell);
    });

    await waitFor(() => {
      expect(createPtyProcess).toHaveBeenCalledTimes(2);
    });
    expect(createPtyProcess).toHaveBeenLastCalledWith(
      expect.objectContaining({
        kind: 'pty.shell',
        shellId: selectedShell.id,
      })
    );
  }
);

test('open new tab', async () => {
  const { docsService } = await getTestSetup({
    documents: [getMockDocuments()[0]],
  });
  const { add, open } = docsService;
  // Use a URI of a cluster that's not in ClustersService so that DocumentCluster doesn't render
  // UnifiedResources for it. UnifiedResources requires a lot of mocks to be set up.
  const nonExistentClusterUri = routing.getClusterUri({
    ...routing.parseClusterUri(rootClusterUri).params,
    leafClusterId: 'nonexistent-leaf',
  });
  const mockedClusterDocument = makeDocumentCluster({
    clusterUri: nonExistentClusterUri,
  });
  docsService.createClusterDocument = () => mockedClusterDocument;
  const $newTabButton = screen.getByTitle('New Tab', { exact: false });

  fireEvent.click($newTabButton);

  expect(add).toHaveBeenCalledWith(mockedClusterDocument);
  expect(open).toHaveBeenCalledWith(mockedClusterDocument.uri);
});

test('swap tabs', async () => {
  const { docsService } = await getTestSetup({
    documents: getMockDocuments(),
  });
  const documents = docsService.getDocuments();
  const $firstTab = screen.getByTitle(documents[0].title);
  const $secondTab = screen.getByTitle(documents[1].title);

  fireEvent.dragStart($secondTab);
  fireEvent.drop($firstTab);

  expect(docsService.swapPosition).toHaveBeenCalledWith(1, 0);
});
