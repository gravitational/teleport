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

import { useCallback, useEffect } from 'react';

import { Gateway } from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';
import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import {
  DocumentGateway,
  getDocumentGatewayTitle,
} from 'teleterm/ui/services/workspacesService';
import { isAppUri, isDatabaseUri } from 'teleterm/ui/uri';
import { retryWithRelogin } from 'teleterm/ui/utils';

export function useGateway(doc: DocumentGateway) {
  const ctx = useAppContext();
  const { clustersService, usageService } = ctx;
  const { documentsService } = useWorkspaceContext();
  // The port to show as default in the input field in case creating a gateway fails.
  // This is typically the case if someone reopens the app and the port of the gateway is already
  // occupied.
  //
  // This needs a default value as otherwise React will complain about switching an uncontrolled
  // input to a controlled one once `doc.port` gets set. The backend will handle converting an empty
  // string to '0'.
  const defaultPort = doc.port || '';
  const gateway = useStoreSelector(
    'clustersService',
    useCallback(state => state.gateways.get(doc.gatewayUri), [doc.gatewayUri])
  );
  const connected = !!gateway;

  const [connectAttempt, createGateway] = useAsync(
    useCallback(
      async (args: { localPort?: string; targetSubresourceName?: string }) => {
        documentsService.update(doc.uri, { status: 'connecting' });
        let gw: Gateway;

        try {
          gw = await retryWithRelogin(ctx, doc.targetUri, () =>
            clustersService.createGateway({
              targetUri: doc.targetUri,
              localPort: args.localPort,
              targetUser: doc.targetUser,
              targetSubresourceName:
                args.targetSubresourceName || doc.targetSubresourceName,
            })
          );
        } catch (error) {
          documentsService.update(doc.uri, { status: 'error' });
          throw error;
        }
        documentsService.update(doc.uri, draft => {
          const draftDoc = draft as DocumentGateway;
          draftDoc.gatewayUri = gw.uri;
          // Set the port on doc to match the one returned from the daemon. By default,
          // createGateway is called with an empty localPort, so the daemon creates a listener on a
          // random port.
          //
          // Setting it here makes it so that on app restart, Teleterm will restart the proxy with the
          // same port number.
          //
          // Alternatively, if createGateway was called from OfflineGateway, this will persist in
          // the doc the local port chosen by the user.
          draftDoc.port = gw.localPort;
          // targetSubresourceName needs to be updated here in case the createGateway function was
          // called from OfflineGateway.
          draftDoc.targetSubresourceName = gw.targetSubresourceName;
          draftDoc.status = 'connected';
          // The title might need to be changed if OfflineGateway changed gateway params.
          draftDoc.title = getDocumentGatewayTitle(draftDoc);
        });
        if (isDatabaseUri(doc.targetUri)) {
          usageService.captureProtocolUse({
            uri: doc.targetUri,
            protocol: 'db',
            origin: doc.origin,
            accessThrough: 'local_proxy',
          });
        }
        if (isAppUri(doc.targetUri)) {
          usageService.captureProtocolUse({
            uri: doc.targetUri,
            protocol: 'app',
            origin: doc.origin,
            accessThrough: 'local_proxy',
          });
        }
      },
      [clustersService, ctx, doc, documentsService, usageService]
    )
  );

  const [disconnectAttempt, disconnect] = useAsync(async () => {
    await clustersService.removeGateway(doc.gatewayUri);
    documentsService.close(doc.uri);
  });

  const [changeTargetSubresourceNameAttempt, changeTargetSubresourceName] =
    useAsync(
      useCallback(
        (name: string) =>
          retryWithRelogin(ctx, doc.targetUri, async () => {
            const updatedGateway =
              await clustersService.setGatewayTargetSubresourceName(
                doc.gatewayUri,
                name
              );

            documentsService.update(doc.uri, draft => {
              const draftDoc = draft as DocumentGateway;

              draftDoc.targetSubresourceName =
                updatedGateway.targetSubresourceName;
              draftDoc.title = getDocumentGatewayTitle(draftDoc);
            });
          }),
        [
          clustersService,
          documentsService,
          doc.uri,
          doc.gatewayUri,
          ctx,
          doc.targetUri,
        ]
      )
    );

  const [changePortAttempt, changePort] = useAsync(
    useCallback(
      async (port: string) => {
        const updatedGateway = await clustersService.setGatewayLocalPort(
          doc.gatewayUri,
          port
        );

        documentsService.update(doc.uri, {
          targetSubresourceName: updatedGateway.targetSubresourceName,
          port: updatedGateway.localPort,
        });
      },
      [clustersService, documentsService, doc.uri, doc.gatewayUri]
    )
  );

  useEffect(
    function createGatewayOnMount() {
      // Since the user can close DocumentGateway without shutting down the gateway, it's possible
      // to open DocumentGateway while the gateway is already running. In that scenario, we must
      // not attempt to create a gateway.
      if (!gateway && connectAttempt.status === '') {
        createGateway({ localPort: doc.port });
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
