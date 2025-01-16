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

import { Meta } from '@storybook/react';
import { useState } from 'react';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { Document } from 'teleterm/ui/services/workspacesService';
import {
  makeDocumentAccessRequests,
  makeDocumentAuthorizeWebSession,
  makeDocumentCluster,
  makeDocumentConnectMyComputer,
  makeDocumentGatewayApp,
  makeDocumentGatewayCliClient,
  makeDocumentGatewayDatabase,
  makeDocumentGatewayKube,
  makeDocumentPtySession,
  makeDocumentTshNode,
} from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';
import { Tabs } from 'teleterm/ui/Tabs/Tabs';

const meta: Meta = {
  title: 'Teleterm/Tabs',
};

export default meta;

const allDocuments: Document[] = [
  makeDocumentCluster(),
  makeDocumentTshNode(),
  makeDocumentConnectMyComputer(),
  makeDocumentGatewayDatabase(),
  makeDocumentGatewayApp(),
  makeDocumentGatewayCliClient(),
  makeDocumentGatewayKube(),
  makeDocumentAccessRequests(),
  makeDocumentPtySession(),
  makeDocumentAuthorizeWebSession(),
];

export function Story() {
  const [activeTab, setActiveTab] = useState(allDocuments[0].uri);
  return (
    <MockAppContextProvider>
      <Tabs
        items={allDocuments}
        activeTab={activeTab}
        onMoved={() => {}}
        closeTabTooltip=""
        newTabTooltip=""
        onNew={() => {}}
        onContextMenu={() => {}}
        onSelect={d => setActiveTab(d.uri)}
      />
    </MockAppContextProvider>
  );
}
