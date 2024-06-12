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

import { useEffect } from 'react';

import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import * as types from 'teleterm/ui/services/workspacesService';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { retryWithRelogin } from 'teleterm/ui/utils';
import * as tshdGateway from 'teleterm/services/tshd/gateway';
import { Gateway } from 'teleterm/services/tshd/types';
import { isDatabaseUri, isAppUri } from 'teleterm/ui/uri';

export function useGateway(doc: types.DocumentGateway) {
  const ctx = useAppContext();
  const { documentsService: workspaceDocumentsService } = useWorkspaceContext();
  // The port to show as default in the input field in case creating a gateway fails.
  // This is typically the case if someone reopens the app and the port of the gateway is already
  // occupied.
  //
  // This needs a default value as otherwise React will complain about switching an uncontrolled
  // input to a controlled one once `doc.port` gets set. The backend will handle converting an empty
  // string to '0'.
  const defaultPort = doc.port || '';
  const gateway = ctx.clustersService.findGateway(doc.gatewayUri);
  const connected = !!gateway;

  const [connectAttempt, createGateway] = useAsync(async (port: string) => {
    workspaceDocumentsService.update(doc.uri, { status: 'connecting' });
    let gw: Gateway;

    try {
      gw = await retryWithRelogin(ctx, doc.targetUri, () =>
        ctx.clustersService.createGateway({
          targetUri: doc.targetUri,
          localPort: port,
          targetUser: doc.targetUser,
          targetSubresourceName: doc.targetSubresourceName,
        })
      );
    } catch (error) {
      workspaceDocumentsService.update(doc.uri, { status: 'error' });
      throw error;
    }
    workspaceDocumentsService.update(doc.uri, {
      gatewayUri: gw.uri,
      // Set the port on doc to match the one returned from the daemon. Teleterm doesn't let the
      // user provide a port for the gateway, so instead we have to let the daemon use a random
      // one.
      //
      // Setting it here makes it so that on app restart, Teleterm will restart the proxy with the
      // same port number.
      port: gw.localPort,
      status: 'connected',
    });
    if (isDatabaseUri(doc.targetUri)) {
      ctx.usageService.captureProtocolUse(doc.targetUri, 'db', doc.origin);
    }
    if (isAppUri(doc.targetUri)) {
      ctx.usageService.captureProtocolUse(doc.targetUri, 'app', doc.origin);
    }
  });

  const [disconnectAttempt, disconnect] = useAsync(async () => {
    await ctx.clustersService.removeGateway(doc.gatewayUri);
    workspaceDocumentsService.close(doc.uri);
  });

  const [changeTargetSubresourceNameAttempt, changeTargetSubresourceName] =
    useAsync(async (name: string) => {
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

  useEffect(
    function createGatewayOnMount() {
      // Since the user can close DocumentGateway without shutting down the gateway, it's possible
      // to open DocumentGateway while the gateway is already running. In that scenario, we must
      // not attempt to create a gateway.
      if (!gateway && connectAttempt.status === '') {
        createGateway(doc.port);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  return {
    gateway,
    defaultPort,
    disconnect,
    connected,
    reconnect: createGateway,
    connectAttempt,
    disconnectAttempt,
    changeTargetSubresourceName,
    changeTargetSubresourceNameAttempt,
    changePort,
    changePortAttempt,
  };
}

//TODO(gzdunek): Refactor DocumentGateway so the hook below is no longer needed.
// We should move away from using one big hook per component.
export function useDocumentGateway(doc: types.DocumentGateway) {
  const { documentsService: workspaceDocumentsService } = useWorkspaceContext();

  const {
    gateway,
    reconnect,
    connectAttempt,
    disconnectAttempt,
    disconnect,
    connected,
    changePort,
    changePortAttempt,
    changeTargetSubresourceNameAttempt,
    changeTargetSubresourceName,
    defaultPort,
  } = useGateway(doc);

  const runCliCommand = () => {
    if (!isDatabaseUri(doc.targetUri)) {
      return;
    }
    const command = tshdGateway.getCliCommandArgv0(gateway.gatewayCliCommand);
    const title = `${command} · ${doc.targetUser}@${doc.targetName}`;

    const cliDoc = workspaceDocumentsService.createGatewayCliDocument({
      title,
      targetUri: doc.targetUri,
      targetUser: doc.targetUser,
      targetName: doc.targetName,
      targetProtocol: gateway.protocol,
    });
    workspaceDocumentsService.add(cliDoc);
    workspaceDocumentsService.setLocation(cliDoc.uri);
  };

  return {
    reconnect,
    connectAttempt,
    // TODO(ravicious): Show disconnectAttempt errors in UI.
    disconnectAttempt,
    disconnect,
    connected,
    gateway,
    changeDbNameAttempt: changeTargetSubresourceNameAttempt,
    changePort,
    changePortAttempt,
    changeDbName: changeTargetSubresourceName,
    defaultPort,
    runCliCommand,
  };
}
