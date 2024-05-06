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

import { createRef } from 'react';
import { fireEvent, render, screen } from 'design/utils/testing';

import { TabHost } from 'teleterm/ui/TabHost/TabHost';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { Document } from 'teleterm/ui/services/workspacesService';
import { TabContextMenuOptions } from 'teleterm/mainProcess/types';
import { makeDocumentCluster } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

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

const rootClusterUri = '/clusters/test_uri';

function getTestSetup({ documents }: { documents: Document[] }) {
  const appContext = new MockAppContext();
  jest.spyOn(appContext.mainProcessClient, 'openTabContextMenu');

  appContext.clustersService.setState(draft => {
    draft.clusters.set(
      rootClusterUri,
      makeRootCluster({
        uri: rootClusterUri,
      })
    );
  });

  appContext.workspacesService.setState(draft => {
    draft.rootClusterUri = rootClusterUri;
    draft.workspaces[rootClusterUri] = {
      documents,
      location: documents[0]?.uri,
      localClusterUri: rootClusterUri,
      accessRequests: undefined,
    };
  });

  const docsService =
    appContext.workspacesService.getActiveWorkspaceDocumentService();

  jest.spyOn(docsService, 'add');
  jest.spyOn(docsService, 'open');
  jest.spyOn(docsService, 'close');
  jest.spyOn(docsService, 'swapPosition');
  jest.spyOn(docsService, 'closeOthers');
  jest.spyOn(docsService, 'closeToRight');
  jest.spyOn(docsService, 'duplicatePtyAndActivate');

  const utils = render(
    <MockAppContextProvider appContext={appContext}>
      <TabHost ctx={appContext} topBarContainerRef={createRef()} />
    </MockAppContextProvider>
  );

  return {
    ...utils,
    docsService,
    mainProcessClient: appContext.mainProcessClient,
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
