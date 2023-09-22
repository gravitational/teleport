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
import { act, renderHook } from '@testing-library/react-hooks';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

import {
  ResourcesContextProvider,
  useResourcesContext,
} from '../resourcesContext';
import ClusterContext, { ClusterContextProvider } from '../clusterContext';

import { useServerSideResources } from './useServerSideResources';

beforeEach(() => {
  jest.restoreAllMocks();
});

it('adds a listener for resource refresh', async () => {
  let requestResourcesRefresh: () => void;
  const fetchFunction = jest.fn(() =>
    Promise.resolve({
      agentsList: [],
      totalCount: 0,
      startKey: '',
    })
  );
  const appContext = new MockAppContext();
  const cluster = makeRootCluster();

  // We must somehow trigger a request for resources refresh. Since we don't mock anything, we have
  // to use capture requestResourcesRefresh somehow.
  const RefreshRequester = () => {
    requestResourcesRefresh = useResourcesContext().requestResourcesRefresh;

    return null;
  };
  const wrapper = ({ children }) => (
    <MockAppContextProvider appContext={appContext}>
      <ClusterContextProvider
        value={new ClusterContext(appContext, cluster.uri, '/docs/123')}
      >
        <ResourcesContextProvider>
          {children}

          <RefreshRequester />
        </ResourcesContextProvider>
      </ClusterContextProvider>
    </MockAppContextProvider>
  );

  const { result, waitFor, unmount } = renderHook(
    () =>
      useServerSideResources<any>(
        { fieldName: 'hostname', dir: 'ASC' },
        fetchFunction
      ),
    { wrapper }
  );

  expect(result.error).toBeFalsy();

  // Wait for the initial fetch to finish.
  await waitFor(() => result.current.fetchAttempt.status === 'success');
  expect(fetchFunction).toHaveBeenCalledTimes(1);

  // Verify that the listener is added.
  act(() => {
    requestResourcesRefresh();
  });
  await waitFor(() => result.current.fetchAttempt.status === 'success');
  expect(fetchFunction).toHaveBeenCalledTimes(2);

  // Verify that the cleanup function gets called.
  unmount();
  requestResourcesRefresh();
  expect(fetchFunction).toHaveBeenCalledTimes(2);
});
