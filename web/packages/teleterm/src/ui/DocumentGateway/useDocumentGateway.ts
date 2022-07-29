/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useEffect } from 'react';

import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import * as types from 'teleterm/ui/services/workspacesService';
import { useWorkspaceDocumentsService } from 'teleterm/ui/Documents';
import { routing } from 'teleterm/ui/uri';
import { retryWithRelogin } from 'teleterm/ui/utils';

export default function useGateway(doc: types.DocumentGateway) {
  const ctx = useAppContext();
  const workspaceDocumentsService = useWorkspaceDocumentsService();
  // The port to show as default in the input field in case creating a gateway fails.
  // This is typically the case if someone reopens the app and the port of the gateway is already
  // occupied.
  const defaultPort = doc.port || '0';
  const gateway = ctx.clustersService.findGateway(doc.gatewayUri);
  const connected = !!gateway;
  const rootCluster = ctx.clustersService.findRootClusterByResource(
    doc.targetUri
  );
  const cluster = ctx.clustersService.findClusterByResource(doc.targetUri);

  const [connectAttempt, createGateway] = useAsync(async (port: string) => {
    const gw = await retryWithRelogin(ctx, doc.uri, doc.targetUri, () =>
      ctx.clustersService.createGateway({
        targetUri: doc.targetUri,
        port: port,
        user: doc.targetUser,
        subresource_name: doc.targetSubresourceName,
      })
    );

    workspaceDocumentsService.update(doc.uri, {
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

  const [disconnectAttempt, disconnect] = useAsync(async () => {
    await ctx.clustersService.removeGateway(doc.gatewayUri);
  });

  const [changeDbNameAttempt, changeDbName] = useAsync(async (name: string) => {
    const updatedGateway =
      await ctx.clustersService.setGatewayTargetSubresourceName(
        doc.gatewayUri,
        name
      );

    workspaceDocumentsService.update(doc.uri, {
      targetSubresourceName: updatedGateway.targetSubresourceName,
    });
  });

  const [changePortAttempt, changePort] = useAsync(async (port: string) => {
    const updatedGateway = await ctx.clustersService.setGatewayLocalPort(
      doc.gatewayUri,
      port
    );

    workspaceDocumentsService.update(doc.uri, {
      targetSubresourceName: updatedGateway.targetSubresourceName,
      port: updatedGateway.localPort,
    });
  });

  const reconnect = (port: string) => {
    if (rootCluster.connected) {
      createGateway(port);
      return;
    }

    ctx.commandLauncher.executeCommand('cluster-connect', {
      clusterUri: rootCluster.uri,
      onSuccess: () => createGateway(doc.port),
    });
  };

  const runCliCommand = () => {
    const { rootClusterId, leafClusterId } = routing.parseClusterUri(
      cluster.uri
    ).params;
    workspaceDocumentsService.openNewTerminal({
      initCommand: gateway.cliCommand,
      rootClusterId,
      leafClusterId,
    });
  };

  useEffect(() => {
    if (disconnectAttempt.status === 'success') {
      workspaceDocumentsService.close(doc.uri);
    }
  }, [disconnectAttempt.status]);

  const shouldCreateGateway =
    rootCluster.connected && !connected && connectAttempt.status === '';

  useEffect(
    function createGatewayOnDocumentOpen() {
      if (shouldCreateGateway) {
        createGateway(doc.port);
      }
    },
    [shouldCreateGateway]
  );

  return {
    gateway,
    defaultPort,
    disconnect,
    connected,
    reconnect,
    connectAttempt,
    runCliCommand,
    changeDbName,
    changeDbNameAttempt,
    changePort,
    changePortAttempt,
  };
}
