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

import { Meta } from '@storybook/react';
import { ComponentProps } from 'react';
import {
  makeEmptyAttempt,
  makeProcessingAttempt,
  makeErrorAttempt,
} from 'shared/hooks/useAsync';

import { makeDatabaseGateway } from 'teleterm/services/tshd/testHelpers';

import { OfflineGateway } from '../components/OfflineGateway';

import { OnlineDocumentGateway } from './OnlineDocumentGateway';

const meta: Meta = {
  title: 'Teleterm/DocumentGateway',
};
export default meta;

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

const onlineDocumentGatewayProps: ComponentProps<typeof OnlineDocumentGateway> =
  {
    gateway: gateway,
    disconnect: async () => [undefined, null],
    runCliCommand: () => {},
    changeDbName: async () => [undefined, null],
    changeDbNameAttempt: makeEmptyAttempt(),
    changePort: async () => [undefined, null],
    changePortAttempt: makeEmptyAttempt(),
  };

export function Online() {
  return <OnlineDocumentGateway {...onlineDocumentGatewayProps} />;
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
    <OnlineDocumentGateway {...onlineDocumentGatewayProps} gateway={gateway} />
  );
}

export function OnlineWithFailedDbNameAttempt() {
  return (
    <OnlineDocumentGateway
      {...onlineDocumentGatewayProps}
      changeDbNameAttempt={makeErrorAttempt(
        new Error('Something went wrong with setting database name.')
      )}
    />
  );
}

export function OnlineWithFailedPortAttempt() {
  return (
    <OnlineDocumentGateway
      {...onlineDocumentGatewayProps}
      changePortAttempt={makeErrorAttempt(
        new Error('Something went wrong with setting port.')
      )}
    />
  );
}

export function OnlineWithFailedDbNameAndPortAttempts() {
  return (
    <OnlineDocumentGateway
      {...onlineDocumentGatewayProps}
      changeDbNameAttempt={makeErrorAttempt(
        new Error('Something went wrong with setting database name.')
      )}
      changePortAttempt={makeErrorAttempt(
        new Error('Something went wrong with setting port.')
      )}
    />
  );
}

const offlineGatewayProps: ComponentProps<typeof OfflineGateway> = {
  connectAttempt: makeEmptyAttempt(),
  reconnect: () => {},
  gatewayPort: { isSupported: true, defaultPort: '1337' },
  targetName: gateway.targetName,
  gatewayKind: 'database',
};

export function Offline() {
  return <OfflineGateway {...offlineGatewayProps} />;
}

export function OfflineWithFailedConnectAttempt() {
  return (
    <OfflineGateway
      {...offlineGatewayProps}
      connectAttempt={makeErrorAttempt(
        new Error('listen tcp 127.0.0.1:62414: bind: address already in use')
      )}
    />
  );
}

export function Processing() {
  return (
    <OfflineGateway
      {...offlineGatewayProps}
      connectAttempt={makeProcessingAttempt()}
    />
  );
}

export function PortProcessing() {
  return (
    <OnlineDocumentGateway
      {...onlineDocumentGatewayProps}
      changePortAttempt={makeProcessingAttempt()}
    />
  );
}
