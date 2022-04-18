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

import React, { useEffect } from 'react';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import * as types from 'teleterm/ui/services/workspacesService';
import useAsync from 'teleterm/ui/useAsync';
import { useWorkspaceDocumentsService } from 'teleterm/ui/Documents';

export default function useGateway(doc: types.DocumentGateway) {
  const ctx = useAppContext();
  const workspaceDocumentsService = useWorkspaceDocumentsService();
  const gateway = ctx.clustersService.findGateway(doc.gatewayUri);
  const connected = !!gateway;
  const cluster = ctx.clustersService.findRootClusterByResource(doc.targetUri);

  const [connectAttempt, createGateway, setConnectAttempt] = useAsync(
    async () => {
      const gw = await ctx.clustersService.createGateway({
        targetUri: doc.targetUri,
        port: doc.port,
        user: doc.targetUser,
      });

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
    }
  );

  const [disconnectAttempt, disconnect] = useAsync(async () => {
    await ctx.clustersService.removeGateway(doc.gatewayUri);
  });

  const reconnect = () => {
    if (cluster?.connected) {
      createGateway();
      return;
    }

    if (cluster && !cluster.connected) {
      ctx.commandLauncher.executeCommand('cluster-connect', {
        clusterUri: cluster.uri,
        onSuccess: createGateway,
      });
      return;
    }

    if (!cluster) {
      setConnectAttempt({
        status: 'error',
        statusText: `unable to resolve cluster for ${doc.targetUri}`,
      });
    }
  };

  const runCliCommand = () => {
    workspaceDocumentsService.openNewTerminal(gateway.cliCommand);
  };

  React.useEffect(() => {
    if (disconnectAttempt.status === 'success') {
      workspaceDocumentsService.close(doc.uri);
    }
  }, [disconnectAttempt.status]);

  useEffect(() => {
    if (cluster.connected) {
      createGateway();
    }
  }, [cluster.connected]);

  return {
    doc,
    gateway,
    disconnect,
    connected,
    reconnect,
    connectAttempt,
    runCliCommand,
  };
}

export type State = ReturnType<typeof useGateway>;
