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

import 'jest-canvas-mock';

import { createRef } from 'react';

import { act, fireEvent, render, screen } from 'design/utils/testing';

import { TabContextMenuOptions } from 'teleterm/mainProcess/types';
import {
  makeRootCluster,
  rootClusterUri,
} from 'teleterm/services/tshd/testHelpers';
import { ResourcesContextProvider } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { Document } from 'teleterm/ui/services/workspacesService';
import { makeDocumentCluster } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';
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

async function getTestSetup({ documents }: { documents: Document[] }) {
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
      accessRequests: {
        isBarCollapsed: true,
        pending: { kind: 'resource', resources: new Map() },
      },
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

  render(
    <MockAppContextProvider appContext={appContext}>
      <ResourcesContextProvider>
        <TabHost ctx={appContext} topBarContainerRef={createRef()} />
      </ResourcesContextProvider>
    </MockAppContextProvider>
  );

  // Mostly a bogus await just so that all useEffects in all of the mounted contexts have time to be
  // processed and not throw an error due to a state update outside of `act`.
  expect(await screen.findByTitle(/New Tab/)).toBeInTheDocument();

  return {
    docsService,
    mainProcessClient: appContext.mainProcessClient,
  };
}

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

  const $tabTitle = screen.getByTitle(documents[0].title);

  fireEvent.contextMenu($tabTitle);
  expect(openTabContextMenu).toHaveBeenCalled();

  // @ts-expect-error `openTabContextMenu` doesn't know about jest
  const options: TabContextMenuOptions = openTabContextMenu.mock.calls[0][0];
  expect(options.document).toEqual(document);

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
