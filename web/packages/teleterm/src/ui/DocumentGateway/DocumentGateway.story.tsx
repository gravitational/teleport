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
} from 'shared/hooks/useAsync';

import { Gateway } from 'teleterm/services/tshd/types';

import { DocumentGateway } from './DocumentGateway';

export default {
  title: 'Teleterm/DocumentGateway',
};

const gateway: Gateway = {
  uri: '/gateways/bar',
  targetName: 'sales-production',
  targetUri: '/clusters/bar/dbs/foo',
  targetUser: 'alice',
  localAddress: 'localhost',
  localPort: '1337',
  protocol: 'postgres',
  cliCommand: 'connect-me-to-db-please',
  targetSubresourceName: 'bar',
};

export function Online() {
  return (
    <DocumentGateway
      gateway={gateway}
      defaultPort={gateway.localPort}
      disconnect={() => Promise.resolve([undefined, null])}
      connected={true}
      reconnect={() => {}}
      connectAttempt={{ status: 'success', data: undefined, statusText: null }}
      runCliCommand={() => {}}
      changeDbName={() => Promise.resolve([undefined, null])}
      changeDbNameAttempt={makeEmptyAttempt()}
      changePort={() => Promise.resolve([undefined, null])}
      changePortAttempt={makeEmptyAttempt()}
    />
  );
}

export function OnlineWithLongValues() {
  const gateway: Gateway = {
    uri: '/gateways/bar',
    targetName: 'sales-production',
    targetUri: '/clusters/bar/dbs/foo',
    targetUser:
      'quux-quuz-foo-bar-quux-quuz-foo-bar-quux-quuz-foo-bar-quux-quuz-foo-bar',
    localAddress: 'localhost',
    localPort: '13337',
    protocol: 'postgres',
    cliCommand:
      'connect-me-to-db-please-baz-quux-quuz-foo-baz-quux-quuz-foo-baz-quux-quuz-foo',
    targetSubresourceName:
      'foo-bar-baz-quux-quuz-foo-bar-baz-quux-quuz-foo-bar-baz-quux-quuz',
  };

  return (
    <DocumentGateway
      gateway={gateway}
      defaultPort={gateway.localPort}
      disconnect={() => Promise.resolve([undefined, null])}
      connected={true}
      reconnect={() => {}}
      connectAttempt={{ status: 'success', data: undefined, statusText: null }}
      runCliCommand={() => {}}
      changeDbName={() => Promise.resolve([undefined, null])}
      changeDbNameAttempt={makeEmptyAttempt()}
      changePort={() => Promise.resolve([undefined, null])}
      changePortAttempt={makeEmptyAttempt()}
    />
  );
}

export function OnlineWithFailedDbNameAttempt() {
  return (
    <DocumentGateway
      gateway={gateway}
      defaultPort={gateway.localPort}
      disconnect={() => Promise.resolve([undefined, null])}
      connected={true}
      reconnect={() => {}}
      connectAttempt={{ status: 'success', data: undefined, statusText: null }}
      runCliCommand={() => {}}
      changeDbName={() => Promise.resolve([undefined, null])}
      changeDbNameAttempt={makeErrorAttempt<void>(
        'Something went wrong with setting database name.'
      )}
      changePort={() => Promise.resolve([undefined, null])}
      changePortAttempt={makeEmptyAttempt()}
    />
  );
}

export function OnlineWithFailedPortAttempt() {
  return (
    <DocumentGateway
      gateway={gateway}
      defaultPort={gateway.localPort}
      disconnect={() => Promise.resolve([undefined, null])}
      connected={true}
      reconnect={() => {}}
      connectAttempt={{ status: 'success', data: undefined, statusText: null }}
      runCliCommand={() => {}}
      changeDbName={() => Promise.resolve([undefined, null])}
      changeDbNameAttempt={makeEmptyAttempt()}
      changePort={() => Promise.resolve([undefined, null])}
      changePortAttempt={makeErrorAttempt<void>(
        'Something went wrong with setting port.'
      )}
    />
  );
}

export function OnlineWithFailedDbNameAndPortAttempts() {
  return (
    <DocumentGateway
      gateway={gateway}
      defaultPort={gateway.localPort}
      disconnect={() => Promise.resolve([undefined, null])}
      connected={true}
      reconnect={() => {}}
      connectAttempt={{ status: 'success', data: undefined, statusText: null }}
      runCliCommand={() => {}}
      changeDbName={() => Promise.resolve([undefined, null])}
      changeDbNameAttempt={makeErrorAttempt<void>(
        'Something went wrong with setting database name.'
      )}
      changePort={() => Promise.resolve([undefined, null])}
      changePortAttempt={makeErrorAttempt<void>(
        'Something went wrong with setting port.'
      )}
    />
  );
}

export function Offline() {
  return (
    <DocumentGateway
      gateway={undefined}
      defaultPort="1337"
      disconnect={() => Promise.resolve([undefined, null])}
      connected={false}
      reconnect={() => {}}
      connectAttempt={makeEmptyAttempt()}
      runCliCommand={() => {}}
      changeDbName={() => Promise.resolve([undefined, null])}
      changeDbNameAttempt={makeEmptyAttempt()}
      changePort={() => Promise.resolve([undefined, null])}
      changePortAttempt={makeEmptyAttempt()}
    />
  );
}

export function OfflineWithFailedConnectAttempt() {
  const connectAttempt = makeErrorAttempt<void>(
    'listen tcp 127.0.0.1:62414: bind: address already in use'
  );

  return (
    <DocumentGateway
      gateway={undefined}
      defaultPort="62414"
      disconnect={() => Promise.resolve([undefined, null])}
      connected={false}
      reconnect={() => {}}
      connectAttempt={connectAttempt}
      runCliCommand={() => {}}
      changeDbName={() => Promise.resolve([undefined, null])}
      changeDbNameAttempt={makeEmptyAttempt()}
      changePort={() => Promise.resolve([undefined, null])}
      changePortAttempt={makeEmptyAttempt()}
    />
  );
}

export function Processing() {
  return (
    <DocumentGateway
      gateway={undefined}
      defaultPort="1337"
      disconnect={() => Promise.resolve([undefined, null])}
      connected={false}
      reconnect={() => {}}
      connectAttempt={makeProcessingAttempt()}
      runCliCommand={() => {}}
      changeDbName={() => Promise.resolve([undefined, null])}
      changeDbNameAttempt={makeEmptyAttempt()}
      changePort={() => Promise.resolve([undefined, null])}
      changePortAttempt={makeEmptyAttempt()}
    />
  );
}
