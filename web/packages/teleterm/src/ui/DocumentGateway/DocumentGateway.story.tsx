import React from 'react';

import {
  makeEmptyAttempt,
  makeErrorAttempt,
  makeProcessingAttempt,
} from 'shared/hooks/useAsync';

import { Gateway } from 'teleterm/services/tshd/types';

import { OfflineDocumentContainer } from 'teleterm/ui/DocumentGateway/Offline/OfflineDocumentContainer';
import { Status } from 'teleterm/ui/DocumentGateway/Offline/Status';
import { ReconnectForm } from 'teleterm/ui/DocumentGateway/Offline/ReconnectForm';
import { Header } from 'teleterm/ui/DocumentGateway/Online/Header';
import { OnlineDocumentContainer } from 'teleterm/ui/DocumentGateway/Online/OnlineDocumentContainer';
import { GUIInstructions } from 'teleterm/ui/DocumentGateway/Online/GUIInstructions';
import { Errors } from 'teleterm/ui/DocumentGateway/Online/Errors';
import { DatabaseForm } from 'teleterm/ui/DocumentGateway/Online/DatabaseForm';
import { CliCommand } from 'teleterm/ui/DocumentGateway/CliCommand';
import { CLIInstructions } from 'teleterm/ui/DocumentGateway/Online/CLIInstructions';

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
    <OnlineDocumentContainer>
      <Header onClose={() => null} />

      <CLIInstructions>
        <DatabaseForm
          dbName={gateway.targetSubresourceName}
          port={gateway.localPort}
          onDbNameChange={() => null}
          onPortChange={() => null}
        />

        <CliCommand
          cliCommand={gateway.cliCommand}
          isLoading={false}
          onRun={() => null}
        />
      </CLIInstructions>

      <GUIInstructions gateway={gateway} />
    </OnlineDocumentContainer>
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
    <OnlineDocumentContainer>
      <Header onClose={() => null} />

      <CLIInstructions>
        <DatabaseForm
          dbName={gateway.targetSubresourceName}
          port={gateway.localPort}
          onDbNameChange={() => null}
          onPortChange={() => null}
        />

        <CliCommand
          cliCommand={gateway.cliCommand}
          isLoading={false}
          onRun={() => null}
        />
      </CLIInstructions>

      <GUIInstructions gateway={gateway} />
    </OnlineDocumentContainer>
  );
}

export function OnlineWithFailedDbNameAttempt() {
  return (
    <OnlineDocumentContainer>
      <Header onClose={() => null} />

      <CLIInstructions>
        <DatabaseForm
          dbName={gateway.targetSubresourceName}
          port={gateway.localPort}
          onDbNameChange={() => null}
          onPortChange={() => null}
        />

        <CliCommand
          cliCommand={gateway.cliCommand}
          isLoading={false}
          onRun={() => null}
        />

        <Errors
          dbNameAttempt={makeErrorAttempt<void>(
            'Something went wrong with setting database name.'
          )}
          portAttempt={makeEmptyAttempt()}
        />
      </CLIInstructions>

      <GUIInstructions gateway={gateway} />
    </OnlineDocumentContainer>
  );
}

export function OnlineWithFailedPortAttempt() {
  return (
    <OnlineDocumentContainer>
      <Header onClose={() => null} />

      <CLIInstructions>
        <DatabaseForm
          dbName={gateway.targetSubresourceName}
          port={gateway.localPort}
          onDbNameChange={() => null}
          onPortChange={() => null}
        />

        <CliCommand
          cliCommand={gateway.cliCommand}
          isLoading={false}
          onRun={() => null}
        />

        <Errors
          dbNameAttempt={makeEmptyAttempt()}
          portAttempt={makeErrorAttempt<void>(
            'Something went wrong with setting port.'
          )}
        />
      </CLIInstructions>

      <GUIInstructions gateway={gateway} />
    </OnlineDocumentContainer>
  );
}

export function OnlineWithFailedDbNameAndPortAttempts() {
  return (
    <OnlineDocumentContainer>
      <Header onClose={() => null} />

      <CLIInstructions>
        <DatabaseForm
          dbName={gateway.targetSubresourceName}
          port={gateway.localPort}
          onDbNameChange={() => null}
          onPortChange={() => null}
        />

        <CliCommand
          cliCommand={gateway.cliCommand}
          isLoading={false}
          onRun={() => null}
        />

        <Errors
          dbNameAttempt={makeErrorAttempt<void>(
            'Something went wrong with setting database name.'
          )}
          portAttempt={makeErrorAttempt<void>(
            'Something went wrong with setting port.'
          )}
        />
      </CLIInstructions>

      <GUIInstructions gateway={gateway} />
    </OnlineDocumentContainer>
  );
}

export function Offline() {
  const connectAttempt = makeEmptyAttempt<void>();

  return (
    <OfflineDocumentContainer>
      <Status attempt={connectAttempt} />
    </OfflineDocumentContainer>
  );
}

export function OfflineWithFailedConnectAttempt() {
  const connectAttempt = makeErrorAttempt<void>(
    'listen tcp 127.0.0.1:62414: bind: address already in use'
  );

  return (
    <OfflineDocumentContainer>
      <Status attempt={connectAttempt} />

      <ReconnectForm
        onSubmit={() => null}
        port="62414"
        showPortInput={true}
        disabled={false}
      />
    </OfflineDocumentContainer>
  );
}

export function Processing() {
  const connectAttempt = makeProcessingAttempt<void>();

  return (
    <OfflineDocumentContainer>
      <Status attempt={connectAttempt} />

      <ReconnectForm
        onSubmit={() => null}
        port=""
        showPortInput={false}
        disabled={true}
      />
    </OfflineDocumentContainer>
  );
}
