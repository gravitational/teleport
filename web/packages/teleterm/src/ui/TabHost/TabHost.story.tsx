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
import { createRef } from 'react';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { ResourcesContextProvider } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
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
  makeDocumentVnetDiagReport,
} from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';

import { TabHostContainer } from './TabHost';

const meta: Meta = {
  title: 'Teleterm/TabHost',
};

export default meta;

const allDocuments = [
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
  makeDocumentVnetDiagReport(),
];

const cluster = makeRootCluster();

export function Story() {
  const ctx = new MockAppContext();
  ctx.addRootClusterWithDoc(cluster, allDocuments);
  return (
    <MockAppContextProvider appContext={ctx}>
      <ResourcesContextProvider>
        <TabHostContainer
          topBarConnectMyComputerRef={createRef()}
          topBarAccessRequestRef={createRef()}
        />
      </ResourcesContextProvider>
    </MockAppContextProvider>
  );
}

// https://stackoverflow.com/questions/53807517/how-to-test-if-two-types-are-exactly-the-same/73461648#73461648
function assert<T extends never>() {} // eslint-disable-line @typescript-eslint/no-unused-vars
type TypeEqualityGuard<A, B> = Exclude<A, B> | Exclude<B, A>;
type ArrayElement<T> = T extends (infer U)[] ? U : never;

type AllExpectedDocs = Exclude<
  Document,
  // DocumentBlank isn't rendered with other documents in the real app.
  | { kind: 'doc.blank' }
  // Deprecated DocumentTshNodeWithLoginHost.
  | { kind: 'doc.terminal_tsh_node'; loginHost: string }
  // Deprecated DocumentTshKube.
  | { kind: 'doc.terminal_tsh_kube' }
>;
// This is going to raise a type error if allDocuments does not include all expected documents
// defined in Document.
assert<TypeEqualityGuard<ArrayElement<typeof allDocuments>, AllExpectedDocs>>();
