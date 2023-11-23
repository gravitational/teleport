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
import { renderHook } from '@testing-library/react-hooks';

import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { makeDocumentCluster } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';

import { useDocumentClusterSearch } from './useDocumentClusterSearch';

const documentCluster = makeDocumentCluster({
  clusterUri: '/clusters/teleport-a',
});

test('if the cluster document is active, the search results should be shown in it', () => {
  const appContext = new MockAppContext();
  appContext.workspacesService.setState(draftState => {
    draftState.workspaces = {
      '/clusters/teleport-a': {
        localClusterUri: '/clusters/teleport-a',
        location: documentCluster.uri,
        accessRequests: undefined,
        documents: [documentCluster],
      },
    };
    draftState.rootClusterUri = '/clusters/teleport-a';
  });

  const { result } = renderHook(
    () =>
      useDocumentClusterSearch({
        inputValue: 'bar',
        filters: [{ filter: 'resource-type', resourceType: 'db' }],
      }),
    {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    }
  );
  const { current } = result;
  expect(current.clusterUri).toEqual(documentCluster.clusterUri);
  expect(current.resourceKinds).toEqual(['db']);
  expect(current.documentUri).toEqual(documentCluster.uri);
  expect(current.value).toBe('bar');
});

test('if the cluster filter does not match the document cluster, the results should be opened in a new document', () => {
  const appContext = new MockAppContext();
  appContext.workspacesService.setState(draftState => {
    draftState.workspaces = {
      '/clusters/teleport-a': {
        localClusterUri: '/clusters/teleport-a',
        location: '/docs/abc',
        accessRequests: undefined,
        documents: [documentCluster],
      },
    };
    draftState.rootClusterUri = '/clusters/teleport-a';
  });

  const { result } = renderHook(
    () =>
      useDocumentClusterSearch({
        inputValue: 'bar',
        // cluster filter set to a cluster different from the one in the document
        filters: [{ filter: 'cluster', clusterUri: '/clusters/teleport-b' }],
      }),
    {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    }
  );
  const { current } = result;
  expect(current.clusterUri).toBe('/clusters/teleport-b');
  expect(current.resourceKinds).toEqual([]);
  expect(current.documentUri).toBeUndefined();
  expect(current.value).toBe('bar');
});

test('if the cluster document is not active, the results should be opened in a new document', () => {
  const appContext = new MockAppContext();
  appContext.workspacesService.setState(draftState => {
    draftState.workspaces = {
      '/clusters/teleport-a': {
        localClusterUri: '/clusters/teleport-a',
        location: '/docs/abc',
        accessRequests: undefined,
        documents: [],
      },
    };
    draftState.rootClusterUri = '/clusters/teleport-a';
  });

  const { result } = renderHook(
    () =>
      useDocumentClusterSearch({
        inputValue: 'bar',
        filters: [{ filter: 'resource-type', resourceType: 'db' }],
      }),
    {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    }
  );
  const { current } = result;
  expect(current.clusterUri).toBe('/clusters/teleport-a');
  expect(current.resourceKinds).toEqual(['db']);
  expect(current.documentUri).toBeUndefined();
  expect(current.value).toBe('bar');
});
