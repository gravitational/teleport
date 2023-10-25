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
import { act } from '@testing-library/react';
import { render, screen } from 'design/utils/testing';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';

import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import {
  makeRootCluster,
  makeLoggedInUser,
} from 'teleterm/services/tshd/testHelpers';
import * as tsh from 'teleterm/services/tshd/types';
import { ConnectMyComputerContextProvider } from 'teleterm/ui/ConnectMyComputer';

import { ResourcesContextProvider } from './resourcesContext';

import DocumentCluster from './DocumentCluster';

const mio = mockIntersectionObserver();

it('displays a button for Connect My Computer in the empty state if the user can use Connect My Computer', async () => {
  const doc = {
    kind: 'doc.cluster' as const,
    clusterUri: '/clusters/localhost' as const,
    uri: '/docs/123' as const,
    title: 'sample',
  };

  const appContext = new MockAppContext({ platform: 'darwin' });
  appContext.clustersService.setState(draft => {
    draft.clusters.set(
      doc.clusterUri,
      makeRootCluster({
        uri: doc.clusterUri,
        loggedInUser: makeLoggedInUser({
          userType: tsh.UserType.USER_TYPE_LOCAL,
          acl: {
            tokens: {
              create: true,
              list: true,
              edit: true,
              pb_delete: true,
              read: true,
              use: true,
            },
          },
        }),
      })
    );
  });

  appContext.workspacesService.setState(draftState => {
    const rootClusterUri = doc.clusterUri;
    draftState.rootClusterUri = rootClusterUri;
    draftState.workspaces[rootClusterUri] = {
      localClusterUri: doc.clusterUri,
      documents: [doc],
      location: doc.uri,
      accessRequests: undefined,
    };
  });

  const emptyResponse = {
    resources: [],
    totalCount: 0,
    nextKey: '',
  };
  jest
    .spyOn(appContext.resourcesService, 'listUnifiedResources')
    .mockResolvedValue(emptyResponse);

  render(
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider>
        <ResourcesContextProvider>
          <ConnectMyComputerContextProvider rootClusterUri={doc.clusterUri}>
            <DocumentCluster doc={doc} visible={true} />
          </ConnectMyComputerContextProvider>
        </ResourcesContextProvider>
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );

  const scrollTrigger = screen.getByTestId('scroll-trigger');
  act(() => mio.enterNode(scrollTrigger));

  await expect(
    screen.findByRole('button', { name: 'Connect My Computer' })
  ).resolves.toBeInTheDocument();
});

it('does not display a button for Connect My Computer in the empty state if the user cannot use Connect My Computer', async () => {
  const doc = {
    kind: 'doc.cluster' as const,
    clusterUri: '/clusters/localhost' as const,
    uri: '/docs/123' as const,
    title: 'sample',
  };

  const appContext = new MockAppContext({ platform: 'linux' });
  appContext.clustersService.setState(draft => {
    draft.clusters.set(
      doc.clusterUri,
      makeRootCluster({
        uri: doc.clusterUri,
        loggedInUser: makeLoggedInUser({
          userType: tsh.UserType.USER_TYPE_LOCAL,
          acl: {
            tokens: {
              create: false,
              list: true,
              edit: true,
              pb_delete: true,
              read: true,
              use: true,
            },
          },
        }),
      })
    );
  });

  appContext.workspacesService.setState(draftState => {
    const rootClusterUri = doc.clusterUri;
    draftState.rootClusterUri = rootClusterUri;
    draftState.workspaces[rootClusterUri] = {
      localClusterUri: doc.clusterUri,
      documents: [doc],
      location: doc.uri,
      accessRequests: undefined,
    };
  });

  const emptyResponse = {
    resources: [],
    totalCount: 0,
    nextKey: '',
  };
  jest
    .spyOn(appContext.resourcesService, 'listUnifiedResources')
    .mockResolvedValue(emptyResponse);

  render(
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider>
        <ResourcesContextProvider>
          <ConnectMyComputerContextProvider rootClusterUri={doc.clusterUri}>
            <DocumentCluster doc={doc} visible={true} />
          </ConnectMyComputerContextProvider>
        </ResourcesContextProvider>
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );

  const scrollTrigger = screen.getByTestId('scroll-trigger');
  act(() => mio.enterNode(scrollTrigger));

  await expect(
    screen.findByText('No Resources Found')
  ).resolves.toBeInTheDocument();

  expect(
    screen.queryByRole('button', { name: 'Connect My Computer' })
  ).not.toBeInTheDocument();
});
