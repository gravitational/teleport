/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useEffect } from 'react';

import { useAsync } from 'shared/hooks/useAsync';

import * as types from 'teleterm/ui/services/workspacesService';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { retryWithRelogin } from 'teleterm/ui/utils';
import Document from 'teleterm/ui/Document';
import { DocumentTerminal } from 'teleterm/ui/DocumentTerminal';
import { routing } from 'teleterm/ui/uri';

import { Reconnect } from './Reconnect';

/**
 * DocumentGatewayKube creates a terminal session that presets KUBECONFIG env
 * var to a kubeconfig that can be used to connect the kube gateway.
 *
 * It first tries to create a kube gateway by calling the clusterService. Once
 * connected, it will render DocumentTerminal.
 *
 * TODO(greedy52) doc.gateway_kube replaces doc.terminal_tsh_kube when opening
 * a new kube tab. However, the old doc.terminal_tsh_kube is kept to handle the
 * case where doc.terminal_tsh_kube tabs are saved on disk by the old version
 * of Teleport Connect and need to be reopen by the new version of Teleport
 * Connect. The old doc.terminal_tsh_kube can be DELETED in the next major
 * version (15.0.0) assuming migration should be done by then. Here is the
 * discussion reference:
 * https://github.com/gravitational/teleport/pull/28312#discussion_r1253214517
 */
export const DocumentGatewayKube = (props: {
  visible: boolean;
  doc: types.DocumentGatewayKube;
}) => {
  const { doc, visible } = props;
  const ctx = useAppContext();
  const { documentsService } = useWorkspaceContext();
  const { params } = routing.parseKubeUri(doc.targetUri);
  const [connectAttempt, createGateway] = useAsync(async () => {
    documentsService.update(doc.uri, { status: 'connecting' });

    try {
      await retryWithRelogin(ctx, doc.targetUri, () =>
        // Creating a kube gateway twice with the same params is a noop. tshd
        // will return the URI of an already existing gateway.
        ctx.clustersService.createGateway({
          targetUri: doc.targetUri,
          user: '',
        })
      );
    } catch (error) {
      documentsService.update(doc.uri, { status: 'error' });
      throw error;
    }
  });

  useEffect(() => {
    if (connectAttempt.status === '') {
      createGateway();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  switch (connectAttempt.status) {
    case 'success': {
      return <DocumentTerminal doc={doc} visible={visible} />;
    }

    case 'error': {
      return (
        <Reconnect
          kubeId={params.kubeId}
          statusText={connectAttempt.statusText}
          reconnect={createGateway}
        />
      );
    }

    default: {
      // Show waiting animation.
      return <Document visible={visible} />;
    }
  }
};
