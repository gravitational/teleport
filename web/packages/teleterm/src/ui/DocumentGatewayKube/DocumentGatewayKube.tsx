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
import Document from 'teleterm/ui/Document';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { DocumentTerminal } from 'teleterm/ui/DocumentTerminal';
import * as types from 'teleterm/ui/services/workspacesService';
import { routing } from 'teleterm/ui/uri';
import { retryWithRelogin } from 'teleterm/ui/utils';

import { emptyFormSchema, OfflineGateway } from '../components/OfflineGateway';

/**
 * DocumentGatewayKube creates a terminal session that presets KUBECONFIG env
 * var to a kubeconfig that can be used to connect the kube gateway.
 *
 * It first tries to create a kube gateway by calling the clusterService. Once
 * connected, it will render DocumentTerminal.
 */
export const DocumentGatewayKube = (props: {
  visible: boolean;
  doc: types.DocumentGatewayKube;
}) => {
  const { doc, visible } = props;
  const ctx = useAppContext();
  const { documentsService } = useWorkspaceContext();
  const { params } = routing.parseKubeUri(doc.targetUri);
  const gateway = ctx.clustersService.findGatewayByConnectionParams({
    targetUri: doc.targetUri,
  });
  const connected = !!gateway;

  const [connectAttempt, createGateway] = useAsync(async () => {
    documentsService.update(doc.uri, { status: 'connecting' });

    try {
      await retryWithRelogin(ctx, doc.targetUri, () =>
        ctx.clustersService.createGateway({
          targetUri: doc.targetUri,
          targetSubresourceName: '',
          targetUser: '',
          localPort: '',
        })
      );
    } catch (error) {
      documentsService.update(doc.uri, { status: 'error' });
      throw error;
    }
  });

  useEffect(
    function createGatewayOnMount() {
      // Only creates a gateway if we don't have it for the given params.
      if (!gateway && connectAttempt.status === '') {
        createGateway();
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  if (!connected) {
    return (
      <Document visible={visible}>
        <OfflineGateway
          connectAttempt={connectAttempt}
          targetName={params.kubeId}
          gatewayKind="kube"
          formSchema={emptyFormSchema}
          reconnect={createGateway}
        />
      </Document>
    );
  }

  return <DocumentTerminal doc={doc} visible={visible} />;
};
