/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import React, { useEffect, useCallback } from 'react';

import { useAsync } from 'shared/hooks/useAsync';

import { retryWithRelogin } from 'teleterm/ui/utils';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { DocumentGatewayApp } from 'teleterm/ui/services/workspacesService';
import Document from 'teleterm/ui/Document';
import { Gateway } from 'teleterm/services/tshd/types';
import { OfflineDocumentGateway } from 'teleterm/ui/DocumentGateway/OfflineDocumentGateway';

import { AppGateway } from './AppGateway';

export function DocumentGatewayApp(props: {
  doc: DocumentGatewayApp;
  visible: boolean;
}) {
  const ctx = useAppContext();
  const { doc } = props;
  const { documentsService } = useWorkspaceContext();

  const gateway = ctx.clustersService.findGateway(doc.gatewayUri);
  ctx.clustersService.useState();

  const [connectAttempt, createGateway] = useAsync(
    useCallback(async () => {
      documentsService.update(doc.uri, { status: 'connecting' });

      let gw: Gateway;
      try {
        gw = await retryWithRelogin(ctx, doc.targetUri, () =>
          ctx.clustersService.createGateway({
            targetUri: doc.targetUri,
            port: doc.port,
            user: '',
          })
        );
      } catch (error) {
        documentsService.update(doc.uri, { status: 'error' });
        throw error;
      }

      documentsService.update(doc.uri, {
        gatewayUri: gw.uri,
        port: gw.localPort,
        status: 'connected',
      });
      ctx.usageService.captureProtocolUse(doc.targetUri, 'app', doc.origin);
    }, [ctx, doc.origin, doc.port, doc.targetUri, doc.uri, documentsService])
  );

  const [disconnectAttempt, disconnect] = useAsync(async () => {
    await ctx.clustersService.removeGateway(doc.gatewayUri);
    documentsService.close(doc.uri);
  });

  useEffect(() => {
    // Since the user can close DocumentGatewayApp without shutting down the gateway, it's possible
    // to open DocumentGatewayApp while the gateway is already running. In that scenario, we must
    // not attempt to create a gateway.
    if (!gateway && connectAttempt.status === '') {
      createGateway();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const [changePortAttempt, changePort] = useAsync(async (port: string) => {
    const updatedGateway = await ctx.clustersService.setGatewayLocalPort(
      doc.gatewayUri,
      port
    );

    documentsService.update(doc.uri, {
      port: updatedGateway.localPort,
    });
  });

  return (
    <Document visible={props.visible}>
      {!gateway ? (
        // TODO(gzdunek): This is a temporary screen.
        //  Replace with `OfflineGateway` from https://github.com/gravitational/teleport/pull/36324
        <OfflineDocumentGateway
          connectAttempt={connectAttempt}
          defaultPort={doc.port}
          reconnect={createGateway}
        />
      ) : (
        <AppGateway
          gateway={gateway}
          onDisconnect={disconnect}
          disconnectAttempt={disconnectAttempt}
          changePort={changePort}
          changePortAttempt={changePortAttempt}
        />
      )}
    </Document>
  );
}
