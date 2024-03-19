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

import { fireEvent, render, screen } from 'design/utils/testing';
import React from 'react';

import { TabHost } from 'teleterm/ui/TabHost/TabHost';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import {
  Document,
  DocumentsService,
  WorkspacesService,
} from 'teleterm/ui/services/workspacesService';
import { KeyboardShortcutsService } from 'teleterm/ui/services/keyboardShortcuts';
import {
  MainProcessClient,
  RuntimeSettings,
  TabContextMenuOptions,
} from 'teleterm/mainProcess/types';
import { ClustersService } from 'teleterm/ui/services/clusters';
import AppContext from 'teleterm/ui/appContext';
import { makeDocumentCluster } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';

import { getEmptyPendingAccessRequest } from '../services/workspacesService/accessRequestsService';

// TODO(ravicious): Remove the mock once a separate entry point for e-teleterm is created.
//
// Mocking out DocumentsRenderer because it imports an e-teleterm component which breaks CI tests
// for the OSS version. The tests here don't test the behavior of DocumentsRenderer so the only
// thing we lose by adding the mock is "smoke tests" of different document kinds.
jest.mock('teleterm/ui/Documents/DocumentsRenderer', () => ({
  DocumentsRenderer: ({ children }) => <>{children}</>,
}));

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

function getTestSetup({ documents }: { documents: Document[] }) {
  const keyboardShortcutsService: Partial<KeyboardShortcutsService> = {
    subscribeToEvents() {},
    unsubscribeFromEvents() {},
    // @ts-expect-error we don't provide entire config
    getShortcutsConfig() {
      return {
        closeTab: 'Command-W',
        newTab: 'Command-T',
        openSearchBar: 'Command-K',
        openConnections: 'Command-P',
        openClusters: 'Command-E',
        openProfiles: 'Command-I',
      };
    },
  };

  const mainProcessClient: Partial<MainProcessClient> = {
    openTabContextMenu: jest.fn(),
    getRuntimeSettings: () => ({}) as RuntimeSettings,
  };

  const docsService: Partial<DocumentsService> = {
    getDocuments(): Document[] {
      return documents;
    },
    getActive() {
      return documents[0];
    },
    close: jest.fn(),
    open: jest.fn(),
    add: jest.fn(),
    closeOthers: jest.fn(),
    closeToRight: jest.fn(),
    openNewTerminal: jest.fn(),
    swapPosition: jest.fn(),
    createClusterDocument: jest.fn(),
    duplicatePtyAndActivate: jest.fn(),
  };

  const clustersService: Partial<ClustersService> = {
    subscribe: jest.fn(),
    unsubscribe: jest.fn(),
    findRootClusterByResource: jest.fn(),
    findCluster: jest.fn(),
    findGateway: jest.fn(),
  };

  const workspacesService: Partial<WorkspacesService> = {
    isDocumentActive(documentUri: string) {
      return documentUri === documents[0].uri;
    },
    getRootClusterUri() {
      return '/clusters/test_uri';
    },
    getWorkspaces() {
      return {};
    },
    getActiveWorkspace() {
      return {
        accessRequests: {
          assumed: {},
          isBarCollapsed: false,
          pending: getEmptyPendingAccessRequest(),
        },
        documents,
        location: undefined,
        localClusterUri: '/clusters/test_uri',
      };
    },
    // @ts-expect-error - using mocks
    getActiveWorkspaceDocumentService() {
      return docsService;
    },
    useState: jest.fn(),
    state: {
      workspaces: {},
      rootClusterUri: '/clusters/test_uri',
    },
  };

  const appContext: AppContext = {
    // @ts-expect-error - using mocks
    keyboardShortcutsService,
    // @ts-expect-error - using mocks
    mainProcessClient,
    // @ts-expect-error - using mocks
    clustersService,
    // @ts-expect-error - using mocks
    workspacesService,
  };

  const utils = render(
    <MockAppContextProvider appContext={appContext}>
      <TabHost ctx={appContext} topBarContainerRef={undefined} />
    </MockAppContextProvider>
  );

  return {
    ...utils,
    docsService,
    mainProcessClient,
  };
}

test('render documents', () => {
  const { docsService } = getTestSetup({
    documents: getMockDocuments(),
  });
  const documents = docsService.getDocuments();

  expect(screen.getByTitle(documents[0].title)).toBeInTheDocument();
  expect(screen.getByTitle(documents[1].title)).toBeInTheDocument();
});

test('open tab on click', () => {
  const { getByTitle, docsService } = getTestSetup({
    documents: [getMockDocuments()[0]],
  });
  const documents = docsService.getDocuments();
  const { open } = docsService;
  const $tabTitle = getByTitle(documents[0].title);

  fireEvent.click($tabTitle);

  expect(open).toHaveBeenCalledWith(documents[0].uri);
});

test('open context menu', () => {
  const { getByTitle, docsService, mainProcessClient } = getTestSetup({
    documents: [getMockDocuments()[0]],
  });
  const { openTabContextMenu } = mainProcessClient;
  const { close, closeOthers, closeToRight, duplicatePtyAndActivate } =
    docsService;
  const documents = docsService.getDocuments();
  const document = documents[0];

  const $tabTitle = getByTitle(documents[0].title);

  fireEvent.contextMenu($tabTitle);
  expect(openTabContextMenu).toHaveBeenCalled();

  // @ts-expect-error `openTabContextMenu` doesn't know about jest
  const options: TabContextMenuOptions = openTabContextMenu.mock.calls[0][0];
  expect(options.documentKind).toBe(document.kind);

  options.onClose();
  expect(close).toHaveBeenCalledWith(document.uri);

  options.onCloseOthers();
  expect(closeOthers).toHaveBeenCalledWith(document.uri);

  options.onCloseToRight();
  expect(closeToRight).toHaveBeenCalledWith(document.uri);

  options.onDuplicatePty();
  expect(duplicatePtyAndActivate).toHaveBeenCalledWith(document.uri);
});

test('open new tab', () => {
  const { getByTitle, docsService } = getTestSetup({
    documents: [getMockDocuments()[0]],
  });
  const { add, open } = docsService;
  const mockedClusterDocument = makeDocumentCluster();
  docsService.createClusterDocument = () => mockedClusterDocument;
  const $newTabButton = getByTitle('New Tab', { exact: false });

  fireEvent.click($newTabButton);

  expect(add).toHaveBeenCalledWith(mockedClusterDocument);
  expect(open).toHaveBeenCalledWith(mockedClusterDocument.uri);
});

test('swap tabs', () => {
  const { getByTitle, docsService } = getTestSetup({
    documents: getMockDocuments(),
  });
  const documents = docsService.getDocuments();
  const $firstTab = getByTitle(documents[0].title);
  const $secondTab = getByTitle(documents[1].title);

  fireEvent.dragStart($secondTab);
  fireEvent.drop($firstTab);

  expect(docsService.swapPosition).toHaveBeenCalledWith(1, 0);
});
