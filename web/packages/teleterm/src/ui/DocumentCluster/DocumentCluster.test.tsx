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

import { act } from '@testing-library/react';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';

import { render, screen } from 'design/utils/testing';

import {
  makeAcl,
  makeLoggedInUser,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import * as tsh from 'teleterm/services/tshd/types';
import { ConnectMyComputerContextProvider } from 'teleterm/ui/ConnectMyComputer';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import { makeDocumentCluster } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';

import DocumentCluster from './DocumentCluster';
import { ResourcesContextProvider } from './resourcesContext';

const mio = mockIntersectionObserver();

it('displays a button for Connect My Computer in the empty state if the user can use Connect My Computer', async () => {
  const doc = makeDocumentCluster();

  const appContext = new MockAppContext({ platform: 'darwin' });
  appContext.addRootClusterWithDoc(
    makeRootCluster({
      uri: doc.clusterUri,
      loggedInUser: makeLoggedInUser({
        userType: tsh.LoggedInUser_UserType.LOCAL,
        acl: makeAcl({
          tokens: {
            create: true,
            list: true,
            edit: true,
            delete: true,
            read: true,
            use: true,
          },
        }),
      }),
    }),
    doc
  );

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

  act(mio.enterAll);

  await expect(
    screen.findByRole('button', { name: 'Connect My Computer' })
  ).resolves.toBeInTheDocument();
});

it('does not display a button for Connect My Computer in the empty state if the user cannot use Connect My Computer', async () => {
  const doc = makeDocumentCluster({
    kind: 'doc.cluster' as const,
    clusterUri: '/clusters/localhost' as const,
    uri: '/docs/123' as const,
    title: 'sample',
  });

  const appContext = new MockAppContext({ platform: 'linux' });
  appContext.addRootClusterWithDoc(
    makeRootCluster({
      uri: doc.clusterUri,
      loggedInUser: makeLoggedInUser({
        userType: tsh.LoggedInUser_UserType.LOCAL,
        acl: makeAcl({
          tokens: {
            create: false,
            list: true,
            edit: true,
            delete: true,
            read: true,
            use: true,
          },
        }),
      }),
    }),
    doc
  );

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

  act(mio.enterAll);

  await expect(
    screen.findByText('No Resources Found')
  ).resolves.toBeInTheDocument();

  expect(
    screen.queryByRole('button', { name: 'Connect My Computer' })
  ).not.toBeInTheDocument();
});
