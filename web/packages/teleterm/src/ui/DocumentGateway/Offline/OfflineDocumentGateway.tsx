import React, { useEffect } from 'react';

import { useAsync } from 'shared/hooks/useAsync';

import { retryWithRelogin } from 'teleterm/ui/utils';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import * as types from 'teleterm/ui/services/workspacesService';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { Status } from 'teleterm/ui/DocumentGateway/Offline/Status';

import { ReconnectForm } from 'teleterm/ui/DocumentGateway/Offline/ReconnectForm';

import { OfflineDocumentContainer } from './OfflineDocumentContainer';

interface OfflineDocumentGatewayProps {
  doc: types.DocumentGateway;
}

export function OfflineDocumentGateway(props: OfflineDocumentGatewayProps) {
  const ctx = useAppContext();

  const rootCluster = ctx.clustersService.findRootClusterByResource(
    props.doc.targetUri
  );

  const { documentsService } = useWorkspaceContext();

  const [connectAttempt, createGateway] = useAsync(async (port: string) => {
    const gw = await retryWithRelogin(ctx, props.doc.targetUri, () =>
      ctx.clustersService.createGateway({
        targetUri: props.doc.targetUri,
        port: port,
        user: props.doc.targetUser,
        subresource_name: props.doc.targetSubresourceName,
      })
    );

    documentsService.update(props.doc.uri, {
      gatewayUri: gw.uri,
      // Set the port on doc to match the one returned from the daemon. Teleterm doesn't let the
      // user provide a port for the gateway, so instead we have to let the daemon use a random
      // one.
      //
      // Setting it here makes it so that on app restart, Teleterm will restart the proxy with the
      // same port number.
      port: gw.localPort,
    });
  });

  const reconnect = (port: string) => {
    if (rootCluster.connected) {
      createGateway(port);
      return;
    }

    ctx.commandLauncher.executeCommand('cluster-connect', {
      clusterUri: rootCluster.uri,
      onSuccess: () => createGateway(props.doc.port),
    });
  };

  const shouldCreateGateway =
    rootCluster.connected && connectAttempt.status === '';

  useEffect(
    function createGatewayOnDocumentOpen() {
      if (shouldCreateGateway) {
        createGateway(props.doc.port);
      }
    },
    [shouldCreateGateway]
  );

  const defaultPort = props.doc.port || '';

  const isProcessing = connectAttempt.status === 'processing';
  const showPortInput = connectAttempt.status === 'error';

  return (
    <OfflineDocumentContainer>
      <Status attempt={connectAttempt} />

      <ReconnectForm
        onSubmit={reconnect}
        port={defaultPort}
        showPortInput={showPortInput}
        disabled={isProcessing}
      />
    </OfflineDocumentContainer>
  );
}
