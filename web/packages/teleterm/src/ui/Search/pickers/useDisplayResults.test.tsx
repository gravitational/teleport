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

import { renderHook } from '@testing-library/react';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { makeDocumentCluster } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';

import { useDisplayResults } from './useDisplayResults';

const documentCluster = makeDocumentCluster({
  clusterUri: '/clusters/teleport-a',
});

test('if the cluster document is active, the search results should be shown in it', () => {
  const appContext = new MockAppContext();
  appContext.addRootClusterWithDoc(
    makeRootCluster({ uri: '/clusters/teleport-a' }),
    documentCluster
  );

  const { result } = renderHook(
    () =>
      useDisplayResults({
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
  appContext.addRootClusterWithDoc(
    makeRootCluster({ uri: '/clusters/teleport-a' }),
    documentCluster
  );

  const { result } = renderHook(
    () =>
      useDisplayResults({
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
  appContext.addRootCluster(makeRootCluster({ uri: '/clusters/teleport-a' }));

  const { result } = renderHook(
    () =>
      useDisplayResults({
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
