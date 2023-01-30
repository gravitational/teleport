import React, { useEffect, useMemo } from 'react';
import { debounce } from 'lodash';

import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import * as types from 'teleterm/ui/services/workspacesService';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { Gateway } from 'teleterm/services/tshd/types';
import { OnlineDocumentContainer } from 'teleterm/ui/DocumentGateway/Online/OnlineDocumentContainer';
import { GUIInstructions } from 'teleterm/ui/DocumentGateway/Online/GUIInstructions';
import { CLIInstructions } from 'teleterm/ui/DocumentGateway/Online/CLIInstructions';
import { Header } from 'teleterm/ui/DocumentGateway/Online/Header';
import { Errors } from 'teleterm/ui/DocumentGateway/Online/Errors';
import { routing } from 'teleterm/ui/uri';

interface OnlineDocumentGatewayProps {
  doc: types.DocumentGateway;
  gateway: Gateway;
}

export function OnlineDocumentGateway(props: OnlineDocumentGatewayProps) {
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

  const [disconnectAttempt, disconnect] = useAsync(async () => {
    await ctx.clustersService.removeGateway(props.doc.gatewayUri);
  });

  useEffect(() => {
    if (disconnectAttempt.status === 'success') {
      documentsService.close(props.doc.uri);
    }
  }, [disconnectAttempt.status, props.doc]);

  const handleChangeDbName = useMemo(
    () => debounce(changeDbName, 150),
    [changeDbName]
  );

  const handleChangePort = useMemo(
    () => debounce(changePort, 1000),
    [changePort]
  );

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

  const isProcessing =
    changeDbNameAttempt.status === 'processing' ||
    changePortAttempt.status === 'processing';

  const hasError =
    changeDbNameAttempt.status === 'error' ||
    changePortAttempt.status === 'error';

  return (
    <OnlineDocumentContainer>
      <Header onClose={disconnect} />

      <CLIInstructions
        onRunCommand={runCliCommand}
        isProcessing={isProcessing}
        gateway={props.gateway}
        onChangePort={handleChangePort}
        onChangeDbName={handleChangeDbName}
      />

      {hasError && (
        <Errors
          dbNameAttempt={changeDbNameAttempt}
          portAttempt={changePortAttempt}
        />
      )}

      <GUIInstructions gateway={props.gateway} />
    </OnlineDocumentContainer>
  );
}
