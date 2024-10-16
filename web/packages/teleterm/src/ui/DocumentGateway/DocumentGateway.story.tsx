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

import React from 'react';

import {
  makeEmptyAttempt,
  makeProcessingAttempt,
  makeErrorAttemptWithStatusText,
  makeSuccessAttempt,
} from 'shared/hooks/useAsync';

import { makeDatabaseGateway } from 'teleterm/services/tshd/testHelpers';

import { DocumentGateway, DocumentGatewayProps } from './DocumentGateway';

export default {
  title: 'Teleterm/DocumentGateway',
};

const gateway = makeDatabaseGateway({
  uri: '/gateways/bar',
  targetName: 'sales-production',
  targetUri: '/clusters/bar/dbs/foo',
  targetUser: 'alice',
  localAddress: 'localhost',
  localPort: '1337',
  protocol: 'postgres',
  targetSubresourceName: 'bar',
});
gateway.gatewayCliCommand.preview = 'connect-me-to-db-please';

const onlineDocumentGatewayProps: DocumentGatewayProps = {
  gateway: gateway,
  targetName: gateway.targetName,
  defaultPort: gateway.localPort,
  disconnect: async () => [undefined, null],
  connected: true,
  reconnect: async () => [undefined, null],
  connectAttempt: makeSuccessAttempt(undefined),
  disconnectAttempt: makeEmptyAttempt(),
  runCliCommand: () => {},
  changeDbName: async () => [undefined, null],
  changeDbNameAttempt: makeEmptyAttempt(),
  changePort: async () => [undefined, null],
  changePortAttempt: makeEmptyAttempt(),
};

export function Online() {
  return <DocumentGateway {...onlineDocumentGatewayProps} />;
}

export function OnlineWithLongValues() {
  const gateway = makeDatabaseGateway({
    uri: '/gateways/bar',
    targetName: 'sales-production',
    targetUri: '/clusters/bar/dbs/foo',
    targetUser:
      'quux-quuz-foo-bar-quux-quuz-foo-bar-quux-quuz-foo-bar-quux-quuz-foo-bar',
    localAddress: 'localhost',
    localPort: '13337',
    protocol: 'postgres',
    targetSubresourceName:
      'foo-bar-baz-quux-quuz-foo-bar-baz-quux-quuz-foo-bar-baz-quux-quuz',
  });
  gateway.gatewayCliCommand.preview =
    'connect-me-to-db-please-baz-quux-quuz-foo-baz-quux-quuz-foo-baz-quux-quuz-foo';

  return (
    <DocumentGateway
      {...onlineDocumentGatewayProps}
      gateway={gateway}
      defaultPort={gateway.localPort}
    />
  );
}

export function OnlineWithFailedDbNameAttempt() {
  return (
    <DocumentGateway
      {...onlineDocumentGatewayProps}
      changeDbNameAttempt={makeErrorAttemptWithStatusText<void>(
        'Something went wrong with setting database name.'
      )}
    />
  );
}

export function OnlineWithFailedPortAttempt() {
  return (
    <DocumentGateway
      {...onlineDocumentGatewayProps}
      changePortAttempt={makeErrorAttemptWithStatusText<void>(
        'Something went wrong with setting port.'
      )}
    />
  );
}

export function OnlineWithFailedDbNameAndPortAttempts() {
  return (
    <DocumentGateway
      {...onlineDocumentGatewayProps}
      changeDbNameAttempt={makeErrorAttemptWithStatusText<void>(
        'Something went wrong with setting database name.'
      )}
      changePortAttempt={makeErrorAttemptWithStatusText<void>(
        'Something went wrong with setting port.'
      )}
    />
  );
}

export function Offline() {
  return (
    <DocumentGateway
      {...onlineDocumentGatewayProps}
      gateway={undefined}
      defaultPort="1337"
      connected={false}
      connectAttempt={makeEmptyAttempt()}
    />
  );
}

export function OfflineWithFailedConnectAttempt() {
  return (
    <DocumentGateway
      {...onlineDocumentGatewayProps}
      gateway={undefined}
      defaultPort="62414"
      connected={false}
      connectAttempt={makeErrorAttemptWithStatusText<void>(
        'listen tcp 127.0.0.1:62414: bind: address already in use'
      )}
    />
  );
}

export function Processing() {
  return (
    <DocumentGateway
      {...onlineDocumentGatewayProps}
      gateway={undefined}
      defaultPort="1337"
      connected={false}
      connectAttempt={makeProcessingAttempt()}
    />
  );
}

export function PortProcessing() {
  return (
    <DocumentGateway
      {...onlineDocumentGatewayProps}
      changePortAttempt={makeProcessingAttempt()}
    />
  );
}
