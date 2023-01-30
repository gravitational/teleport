import React, { useMemo } from 'react';

import { useAsync } from 'shared/hooks/useAsync';

import { debounce } from 'lodash';

import { DatabaseForm } from 'teleterm/ui/DocumentGateway/Online/DatabaseForm';
import * as types from 'teleterm/ui/services/workspacesService';
import { CliCommand } from 'teleterm/ui/DocumentGateway/CliCommand';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { Errors } from 'teleterm/ui/DocumentGateway/Online/Errors';
import { routing } from 'teleterm/ui/uri';

import type { Gateway } from 'teleterm/services/tshd/types';

interface CLIFormProps {
  gateway: Gateway;
  doc: types.DocumentGateway;
}

export function CLIForm(props: CLIFormProps) {
  const ctx = useAppContext();

  const cluster = ctx.clustersService.findClusterByResource(
    props.doc.targetUri
  );

  const { documentsService } = useWorkspaceContext();

  const [changeDbNameAttempt, changeDbName] = useAsync(async (name: string) => {
    const updatedGateway =
      await ctx.clustersService.setGatewayTargetSubresourceName(
        props.doc.gatewayUri,
        name
      );

    documentsService.update(props.doc.uri, {
      targetSubresourceName: updatedGateway.targetSubresourceName,
    });
  });

  const [changePortAttempt, changePort] = useAsync(async (port: string) => {
    const updatedGateway = await ctx.clustersService.setGatewayLocalPort(
      props.doc.gatewayUri,
      port
    );

    documentsService.update(props.doc.uri, {
      targetSubresourceName: updatedGateway.targetSubresourceName,
      port: updatedGateway.localPort,
    });
  });

  const runCliCommand = () => {
    const { rootClusterId, leafClusterId } = routing.parseClusterUri(
      cluster.uri
    ).params;
    documentsService.openNewTerminal({
      initCommand: props.gateway.cliCommand,
      rootClusterId,
      leafClusterId,
    });
  };

  const handleChangeDbName = useMemo(
    () => debounce(changeDbName, 150),
    [changeDbName]
  );

  const handleChangePort = useMemo(
    () => debounce(changePort, 1000),
    [changePort]
  );

  const isProcessing =
    changeDbNameAttempt.status === 'processing' ||
    changePortAttempt.status === 'processing';

  const hasError =
    changeDbNameAttempt.status === 'error' ||
    changePortAttempt.status === 'error';

  return (
    <>
      <DatabaseForm
        dbName={props.gateway.targetSubresourceName}
        port={props.gateway.localPort}
        onDbNameChange={handleChangeDbName}
        onPortChange={handleChangePort}
      />

      <CliCommand
        cliCommand={props.gateway.cliCommand}
        isLoading={isProcessing}
        onRun={runCliCommand}
      />

      {hasError && (
        <Errors
          dbNameAttempt={changeDbNameAttempt}
          portAttempt={changePortAttempt}
        />
      )}
    </>
  );
}
