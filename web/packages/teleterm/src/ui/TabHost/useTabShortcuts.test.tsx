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

import { PropsWithChildren } from 'react';

import renderHook from 'design/utils/renderHook';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import AppContextProvider from 'teleterm/ui/appContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import {
  KeyboardShortcutEvent,
  KeyboardShortcutEventSubscriber,
} from 'teleterm/ui/services/keyboardShortcuts';
import { Document } from 'teleterm/ui/services/workspacesService/documentsService';
import { makeDocumentCluster } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';
import { useTabShortcuts } from 'teleterm/ui/TabHost/useTabShortcuts';

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
    {
      kind: 'doc.blank',
      uri: '/docs/test_uri_3',
      title: 'Test 3',
    },
    {
      kind: 'doc.gateway',
      uri: '/docs/test_uri_4',
      title: 'Test 4',
      gatewayUri: '/gateways/gateway4',
      targetUri: '/clusters/bar/dbs/foobar',
      targetName: 'foobar',
      targetUser: 'foo',
      targetSubresourceName: undefined,
      origin: 'resource_table',
      status: '',
    },
    {
      kind: 'doc.gateway',
      uri: '/docs/test_uri_5',
      title: 'Test 5',
      gatewayUri: '/gateways/gateway5',
      targetUri: '/clusters/bar/dbs/foobar',
      targetName: 'foobar',
      targetUser: 'bar',
      targetSubresourceName: undefined,
      origin: 'resource_table',
      status: '',
    },
    makeDocumentCluster({
      uri: '/docs/test_uri_6',
      title: 'Test 6',
    }),
    makeDocumentCluster({
      uri: '/docs/test_uri_7',
      title: 'Test 7',
    }),
    makeDocumentCluster({
      uri: '/docs/test_uri_8',
      title: 'Test 8',
    }),
    makeDocumentCluster({
      uri: '/docs/test_uri_9',
      title: 'Test 9',
    }),
  ];
}

const rootClusterUri = '/clusters/test_uri';

function getTestSetup({ documents }: { documents: Document[] }) {
  const appContext = new MockAppContext();

  let eventEmitter: KeyboardShortcutEventSubscriber;
  jest
    .spyOn(appContext.keyboardShortcutsService, 'subscribeToEvents')
    .mockImplementation((subscriber: KeyboardShortcutEventSubscriber) => {
      eventEmitter = subscriber;
    });
  jest
    .spyOn(appContext.keyboardShortcutsService, 'unsubscribeFromEvents')
    .mockImplementation(() => {
      eventEmitter = null;
    });

  appContext.addRootClusterWithDoc(makeRootCluster(), documents);

  const docsService =
    appContext.workspacesService.getActiveWorkspaceDocumentService();

  jest.spyOn(docsService, 'open');
  jest.spyOn(docsService, 'close');
  jest.spyOn(docsService, 'add');

  renderHook(
    () =>
      useTabShortcuts({
        documentsService: docsService,
        localClusterUri: rootClusterUri,
      }),
    {
      wrapper: (props: PropsWithChildren) => (
        <AppContextProvider value={appContext}>
          {props.children}
        </AppContextProvider>
      ),
    }
  );

  return {
    emitKeyboardShortcutEvent: eventEmitter,
    docsService,
    keyboardShortcutsService: appContext.keyboardShortcutsService,
  };
}

test.each([
  { action: 'tab1', value: 0 },
  { action: 'tab2', value: 1 },
  { action: 'tab3', value: 2 },
  { action: 'tab4', value: 3 },
  { action: 'tab5', value: 4 },
  { action: 'tab6', value: 5 },
  { action: 'tab7', value: 6 },
  { action: 'tab8', value: 7 },
  { action: 'tab9', value: 8 },
])('open tab using $type shortcut', ({ action, value }) => {
  const { emitKeyboardShortcutEvent, docsService } = getTestSetup({
    documents: getMockDocuments(),
  });

  emitKeyboardShortcutEvent({ action } as KeyboardShortcutEvent);

  expect(docsService.open).toHaveBeenCalledWith(
    docsService.getDocuments()[value].uri
  );
});

test('close active tab', () => {
  const { emitKeyboardShortcutEvent, docsService } = getTestSetup({
    documents: [getMockDocuments()[0]],
  });
  const documentToClose = docsService.getDocuments()[0];
  docsService.getActive = () => documentToClose;

  emitKeyboardShortcutEvent({ action: 'closeTab' });

  expect(docsService.close).toHaveBeenCalledWith(documentToClose.uri);
});

test('should ignore close command if no tabs are open', () => {
  const { emitKeyboardShortcutEvent, docsService } = getTestSetup({
    documents: [],
  });

  emitKeyboardShortcutEvent({ action: 'closeTab' });

  expect(docsService.close).not.toHaveBeenCalled();
});

test('open new tab', () => {
  const { emitKeyboardShortcutEvent, docsService } = getTestSetup({
    documents: [],
  });
  const mockedClusterDocument = makeDocumentCluster();
  docsService.createClusterDocument = () => mockedClusterDocument;
  emitKeyboardShortcutEvent({ action: 'newTab' });

  expect(docsService.add).toHaveBeenCalledWith(mockedClusterDocument);
  expect(docsService.open).toHaveBeenCalledWith(mockedClusterDocument.uri);
});

describe('open next/previous tab', () => {
  test('should open next tab', () => {
    const { emitKeyboardShortcutEvent, docsService } = getTestSetup({
      documents: getMockDocuments(),
    });
    const activeTabIndex = 2;
    docsService.getActive = () => docsService.getDocuments()[activeTabIndex];

    emitKeyboardShortcutEvent({ action: 'nextTab' });

    expect(docsService.open).toHaveBeenCalledWith(
      docsService.getDocuments()[activeTabIndex + 1].uri
    );
  });

  test('open first tab if active is the last one', () => {
    const { emitKeyboardShortcutEvent, docsService } = getTestSetup({
      documents: getMockDocuments(),
    });
    const activeTabIndex = docsService.getDocuments().length - 1;
    docsService.getActive = () => docsService.getDocuments()[activeTabIndex];

    emitKeyboardShortcutEvent({ action: 'nextTab' });

    expect(docsService.open).toHaveBeenCalledWith(
      docsService.getDocuments()[0].uri
    );
  });

  test('open previous tab', () => {
    const { emitKeyboardShortcutEvent, docsService } = getTestSetup({
      documents: getMockDocuments(),
    });
    const activeTabIndex = 2;
    docsService.getActive = () => docsService.getDocuments()[activeTabIndex];

    emitKeyboardShortcutEvent({ action: 'previousTab' });

    expect(docsService.open).toHaveBeenCalledWith(
      docsService.getDocuments()[activeTabIndex - 1].uri
    );
  });

  test('open the last tab if active is the first one', () => {
    const { emitKeyboardShortcutEvent, docsService } = getTestSetup({
      documents: getMockDocuments(),
    });
    const activeTabIndex = 0;
    docsService.getActive = () => docsService.getDocuments()[activeTabIndex];

    emitKeyboardShortcutEvent({ action: 'previousTab' });

    expect(docsService.open).toHaveBeenCalledWith(
      docsService.getDocuments()[docsService.getDocuments().length - 1].uri
    );
  });

  test('do not switch tabs if tabs do not exist', () => {
    const { emitKeyboardShortcutEvent, docsService } = getTestSetup({
      documents: [],
    });
    emitKeyboardShortcutEvent({ action: 'nextTab' });

    expect(docsService.open).not.toHaveBeenCalled();
  });
});
