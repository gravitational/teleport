/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/* eslint-disable jest/no-commented-out-tests */

import React from 'react';
import { fireEvent, render } from 'design/utils/testing';
import { ExpanderClustersPresentational } from './ExpanderClusters';
import { ExpanderClusterState } from './types';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';

test.skip('should render simple and trusted clusters', () => {
  const state: ExpanderClusterState = {
    onAddCluster() {},
    onSyncClusters() {},
    onOpenContextMenu() {},
    onOpen() {},
    items: [
      {
        clusterUri: 'test-uri',
        title: 'Test title',
        connected: false,
        syncing: false,
        active: false,
        leaves: [
          {
            clusterUri: 'trusted-cluster-test-uri',
            title: 'Trusted cluster',
            syncing: false,
            connected: false,
            active: false,
          },
        ],
      },
    ],
  };

  const { getByText } = render(
    <MockAppContextProvider>
      <ExpanderClustersPresentational {...state} />
    </MockAppContextProvider>
  );

  expect(getByText(state.items[0].title)).toBeInTheDocument();
  expect(getByText(state.items[0].leaves[0].title)).toBeInTheDocument();
});

test.skip('should invoke callback when context menu is clicked', () => {
  const state: ExpanderClusterState = {
    onAddCluster() {},
    onSyncClusters() {},
    onOpenContextMenu: jest.fn(),
    onOpen() {},
    items: [
      {
        clusterUri: 'test-uri',
        title: 'Test title',
        connected: false,
        syncing: false,
        active: false,
      },
    ],
  };

  const { getByText } = render(
    <MockAppContextProvider>
      <ExpanderClustersPresentational {...state} />
    </MockAppContextProvider>
  );

  fireEvent.contextMenu(getByText(state.items[0].title));
  expect(state.onOpenContextMenu).toHaveBeenCalledWith(state.items[0]);
});

/*
test('should invoke callback when remove button is clicked', () => {
  const handleRemove = jest.fn();

  const items: ClusterNavItem[] = [
    { uri: 'test-uri', title: 'Test title', connected: false, syncing: false },
  ];
  const { getByTitle } = render(
    <MockAppContextProvider>
      <ExpanderClustersPresentational items={items} onRemove={handleRemove} />
    </MockAppContextProvider>
  );

  const removeButton = getByTitle('Remove');
  expect(removeButton).toBeInTheDocument();

  fireEvent.click(removeButton);
  expect(handleRemove).toHaveBeenCalledWith(items[0].uri);
});

test('should invoke callback when add button is clicked', () => {
  const handleAdd = jest.fn();

  const { getByTitle } = render(
    <MockAppContextProvider>
      <ExpanderClustersPresentational items={[]} onAddCluster={handleAdd} />
    </MockAppContextProvider>
  );

  const addButton = getByTitle('Add cluster');
  expect(addButton).toBeInTheDocument();

  fireEvent.click(addButton);
  expect(handleAdd).toHaveBeenCalledWith();
});

test('should invoke callback when sync button is clicked', () => {
  const handleSync = jest.fn();

  const { getByTitle } = render(
    <MockAppContextProvider>
      <ExpanderClustersPresentational items={[]} onSyncClusters={handleSync} />
    </MockAppContextProvider>
  );

  const syncButton = getByTitle('Sync clusters');
  expect(syncButton).toBeInTheDocument();

  fireEvent.click(syncButton);
  expect(handleSync).toHaveBeenCalledWith();
});
*/
