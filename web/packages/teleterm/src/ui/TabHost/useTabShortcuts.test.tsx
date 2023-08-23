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

import React from 'react';
import renderHook from 'design/utils/renderHook';

import { useTabShortcuts } from 'teleterm/ui/TabHost/useTabShortcuts';
import {
  Document,
  DocumentCluster,
  DocumentsService,
} from 'teleterm/ui/services/workspacesService/documentsService';
import {
  KeyboardShortcutEvent,
  KeyboardShortcutEventSubscriber,
  KeyboardShortcutsService,
} from 'teleterm/ui/services/keyboardShortcuts';
import AppContextProvider from 'teleterm/ui/appContextProvider';
import { WorkspacesService } from 'teleterm/ui/services/workspacesService';
import AppContext from 'teleterm/ui/appContext';

import { getEmptyPendingAccessRequest } from '../services/workspacesService/accessRequestsService';

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
      origin: 'resource_table',
    },
    {
      kind: 'doc.gateway',
      uri: '/docs/test_uri_5',
      title: 'Test 5',
      gatewayUri: '/gateways/gateway5',
      targetUri: '/clusters/bar/dbs/foobar',
      targetName: 'foobar',
      targetUser: 'bar',
      origin: 'resource_table',
    },
    {
      kind: 'doc.cluster',
      uri: '/docs/test_uri_6',
      title: 'Test 6',
      clusterUri: '/clusters/foo',
    },
    {
      kind: 'doc.cluster',
      uri: '/docs/test_uri_7',
      title: 'Test 7',
      clusterUri: '/clusters/test_uri',
    },
    {
      kind: 'doc.cluster',
      uri: '/docs/test_uri_8',
      title: 'Test 8',
      clusterUri: '/clusters/test_uri_8',
    },
    {
      kind: 'doc.cluster',
      uri: '/docs/test_uri_9',
      title: 'Test 9',
      clusterUri: '/clusters/test_uri_9',
    },
  ];
}

function getTestSetup({ documents }: { documents: Document[] }) {
  let eventEmitter: KeyboardShortcutEventSubscriber;
  const keyboardShortcutsService: Partial<KeyboardShortcutsService> = {
    subscribeToEvents(subscriber: KeyboardShortcutEventSubscriber) {
      eventEmitter = subscriber;
    },
    unsubscribeFromEvents() {
      eventEmitter = null;
    },
  };

  // @ts-expect-error - using mocks
  const docsService: DocumentsService = {
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
    duplicatePtyAndActivate: jest.fn(),
  };

  const workspacesService: Partial<WorkspacesService> = {
    getActiveWorkspaceDocumentService() {
      return docsService;
    },
    getActiveWorkspace() {
      return {
        accessRequests: {
          assumed: {},
          isBarCollapsed: false,
          pending: getEmptyPendingAccessRequest(),
        },
        localClusterUri: '/clusters/test_uri',
        documents: [],
        location: '/docs/1',
      };
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
    workspacesService,
  };
  renderHook(
    () =>
      useTabShortcuts({
        documentsService: docsService,
        localClusterUri: workspacesService.getActiveWorkspace().localClusterUri,
      }),
    {
      wrapper: props => (
        <AppContextProvider value={appContext}>
          {props.children}
        </AppContextProvider>
      ),
    }
  );

  return {
    emitKeyboardShortcutEvent: eventEmitter,
    docsService,
    keyboardShortcutsService,
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
  const mockedClusterDocument: DocumentCluster = {
    clusterUri: '/clusters/test',
    uri: '/docs/test',
    title: 'Test',
    kind: 'doc.cluster',
  };
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
