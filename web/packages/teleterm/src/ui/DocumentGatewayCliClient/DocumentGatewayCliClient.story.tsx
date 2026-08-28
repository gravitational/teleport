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

import { DocumentGatewayCliClient } from 'teleterm/ui/services/workspacesService';

import { WaitingForGatewayContent } from './DocumentGatewayCliClient';

export default {
  title: 'Teleterm/DocumentGatewayCliClient',
};

const doc: DocumentGatewayCliClient = {
  uri: '/docs/1234',
  title: 'psql',
  kind: 'doc.gateway_cli_client',
  rootClusterId: 'foo',
  leafClusterId: 'bar',
  status: '',
  targetName: 'postgres',
  targetProtocol: 'postgres',
  targetUri: '/clusters/foo/dbs/elo',
  targetUser: 'alice',
};

const docWithLongValues = {
  ...doc,
  targetName: 'sales-quarterly-fq1-2024-production',
  targetUser:
    'quux-quuz-foo-bar-quux-quuz-foo-bar-quux-quuz-foo-bar-quux-quuz-foo-bar',
};

const noop = () => {};

export const Waiting = (props: { doc?: DocumentGatewayCliClient }) => (
  <WaitingForGatewayContent
    hasTimedOut={false}
    doc={props.doc || doc}
    openConnection={noop}
  />
);

export const WaitingTimedOut = (props: { doc?: DocumentGatewayCliClient }) => (
  <WaitingForGatewayContent
    hasTimedOut={true}
    doc={props.doc || doc}
    openConnection={noop}
  />
);

export const WaitingWithLongValues = () => <Waiting doc={docWithLongValues} />;

export const WaitingTimedOutWithLongValues = () => (
  <WaitingTimedOut doc={docWithLongValues} />
);
