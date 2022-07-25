import React from 'react';

import { makeEmptyAttempt, makeProcessingAttempt } from 'shared/hooks/useAsync';

import { DocumentGateway } from './DocumentGateway';

export default {
  title: 'Teleterm/DocumentGateway',
};

export function Online() {
  const gateway = {
    uri: '/bar',
    targetName: 'sales-production',
    targetUri: '/foo',
    targetUser: 'alice',
    localAddress: 'localhost',
    localPort: '1337',
    protocol: 'postgres',
    cliCommand: 'connect-me-to-db-please',
    targetSubresourceName: 'bar',
  };

  return (
    <DocumentGateway
      gateway={gateway}
      disconnect={() => Promise.resolve([undefined, null])}
      connected={true}
      reconnect={() => {}}
      connectAttempt={{ status: 'success', data: undefined, statusText: null }}
      runCliCommand={() => {}}
      changeDbName={() => Promise.resolve([undefined, null])}
      changeDbNameAttempt={makeEmptyAttempt()}
    />
  );
}

export function Offline() {
  return (
    <DocumentGateway
      gateway={undefined}
      disconnect={() => Promise.resolve([undefined, null])}
      connected={false}
      reconnect={() => {}}
      connectAttempt={makeEmptyAttempt()}
      runCliCommand={() => {}}
      changeDbName={() => Promise.resolve([undefined, null])}
      changeDbNameAttempt={makeEmptyAttempt()}
    />
  );
}

export function Processing() {
  return (
    <DocumentGateway
      gateway={undefined}
      disconnect={() => Promise.resolve([undefined, null])}
      connected={false}
      reconnect={() => {}}
      connectAttempt={makeProcessingAttempt()}
      runCliCommand={() => {}}
      changeDbName={() => Promise.resolve([undefined, null])}
      changeDbNameAttempt={makeEmptyAttempt()}
    />
  );
}

export function OnlineWithLongValues() {
  const gateway = {
    uri: '/bar',
    targetName: 'sales-production',
    targetUri: '/foo',
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
      disconnect={() => Promise.resolve([undefined, null])}
      connected={true}
      reconnect={() => {}}
      connectAttempt={{ status: 'success', data: undefined, statusText: null }}
      runCliCommand={() => {}}
      changeDbName={() => Promise.resolve([undefined, null])}
      changeDbNameAttempt={makeEmptyAttempt()}
    />
  );
}
