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

import Dialog from 'design/Dialog';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import type * as tshd from 'teleterm/services/tshd/types';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import { dialogCss } from '../spacing';
import { ClusterAdd } from './ClusterAdd';

export default {
  title: 'Teleterm/ModalsHost/ClusterAdd',
};

export const Story = () => {
  return (
    <MockAppContextProvider appContext={getMockAppContext()}>
      <Wrapper>
        <ClusterAdd
          prefill={{ clusterAddress: undefined }}
          onSuccess={() => {}}
          onCancel={() => {}}
        />
      </Wrapper>
    </MockAppContextProvider>
  );
};

export const WithPrefill = () => {
  return (
    <MockAppContextProvider appContext={getMockAppContext()}>
      <Wrapper>
        <ClusterAdd
          prefill={{ clusterAddress: 'foo.example.com:3080' }}
          onSuccess={() => {}}
          onCancel={() => {}}
        />
      </Wrapper>
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
      <Wrapper>
        <ClusterAdd
          prefill={{ clusterAddress: undefined }}
          onSuccess={() => {}}
          onCancel={() => {}}
        />
      </Wrapper>
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

const Wrapper = ({ children }: PropsWithChildren) => (
  <Dialog dialogCss={dialogCss} open>
    {children}
  </Dialog>
);
