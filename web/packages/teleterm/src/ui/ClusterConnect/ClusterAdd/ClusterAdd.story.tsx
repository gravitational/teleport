/**
 * Copyright 2020 Gravitational, Inc.
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

import React from 'react';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

import { ClusterAdd } from './ClusterAdd';

import type * as tshd from 'teleterm/services/tshd/types';

export default {
  title: 'Teleterm/ModalsHost/ClusterAdd',
};

export const Story = () => {
  return (
    <MockAppContextProvider appContext={getMockAppContext()}>
      <ClusterAdd
        prefill={{ clusterAddress: undefined }}
        onSuccess={() => {}}
        onCancel={() => {}}
      />
    </MockAppContextProvider>
  );
};

export const WithPrefill = () => {
  return (
    <MockAppContextProvider appContext={getMockAppContext()}>
      <ClusterAdd
        prefill={{ clusterAddress: 'foo.example.com:3080' }}
        onSuccess={() => {}}
        onCancel={() => {}}
      />
    </MockAppContextProvider>
  );
};

export const ErrorOnSubmit = () => {
  return (
    <MockAppContextProvider
      appContext={getMockAppContext({
        addRootCluster: () =>
          Promise.reject(new Error('Oops, something went wrong.')),
      })}
    >
      <ClusterAdd
        prefill={{ clusterAddress: undefined }}
        onSuccess={() => {}}
        onCancel={() => {}}
      />
    </MockAppContextProvider>
  );
};

function getMockAppContext(
  args: {
    addRootCluster?: () => Promise<tshd.Cluster>;
  } = {}
) {
  const appContext = new MockAppContext();
  appContext.clustersService.addRootCluster =
    args.addRootCluster || (() => Promise.resolve(makeRootCluster()));
  return appContext;
}
