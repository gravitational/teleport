/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Meta } from '@storybook/react-vite';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import { makeDocumentVnetInfo } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';
import { ConnectionsContextProvider } from 'teleterm/ui/TopBar/Connections/connectionsContext';

import { DocumentVnetInfo as Component } from './DocumentVnetInfo';
import { VnetContextProvider } from './vnetContext';

const meta: Meta = {
  title: 'Teleterm/Vnet/DocumentVnetInfo',
  decorators: Story => {
    const appCtx = new MockAppContext();
    appCtx.addRootCluster(makeRootCluster());

    return (
      <MockAppContextProvider appContext={appCtx}>
        <ConnectionsContextProvider>
          <MockWorkspaceContextProvider>
            <VnetContextProvider>
              <Story />
            </VnetContextProvider>
          </MockWorkspaceContextProvider>
        </ConnectionsContextProvider>
      </MockAppContextProvider>
    );
  },
};
export default meta;

export const DocumentVnetInfo = () => {
  const doc = makeDocumentVnetInfo();

  return <Component visible doc={doc} />;
};
