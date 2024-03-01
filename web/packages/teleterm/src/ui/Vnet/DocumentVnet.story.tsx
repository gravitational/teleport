/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { rootClusterUri } from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';

import { DocumentVnet } from './DocumentVnet';
import { VnetContextProvider } from './vnetContext';

import type * as docTypes from 'teleterm/ui/services/workspacesService';

export default {
  title: 'Teleterm/Vnet/DocumentVnet',
};

const doc: docTypes.DocumentVnet = {
  kind: 'doc.vnet',
  title: 'VNet',
  rootClusterUri,
  uri: '/docs/1234',
};

export const Story = () => {
  return (
    <MockAppContextProvider>
      <VnetContextProvider>
        <DocumentVnet visible={true} doc={doc} />
      </VnetContextProvider>
    </MockAppContextProvider>
  );
};
