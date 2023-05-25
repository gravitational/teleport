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

import {
  makeEmptyAttempt,
  makeProcessingAttempt,
  makeErrorAttempt,
  makeSuccessAttempt,
} from 'shared/hooks/useAsync';

import { makeGateway } from 'teleterm/services/tshd/testHelpers';

import { DocumentGateway, DocumentGatewayProps } from './DocumentGateway';

export default {
  title: 'Teleterm/DocumentGateway',
};

const gateway = makeGateway({
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
  const gateway = makeGateway({
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
      changeDbNameAttempt={makeErrorAttempt<void>(
        'Something went wrong with setting database name.'
      )}
    />
  );
}

export function OnlineWithFailedPortAttempt() {
  return (
    <DocumentGateway
      {...onlineDocumentGatewayProps}
      changePortAttempt={makeErrorAttempt<void>(
        'Something went wrong with setting port.'
      )}
    />
  );
}

export function OnlineWithFailedDbNameAndPortAttempts() {
  return (
    <DocumentGateway
      {...onlineDocumentGatewayProps}
      changeDbNameAttempt={makeErrorAttempt<void>(
        'Something went wrong with setting database name.'
      )}
      changePortAttempt={makeErrorAttempt<void>(
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
      connectAttempt={makeErrorAttempt<void>(
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
