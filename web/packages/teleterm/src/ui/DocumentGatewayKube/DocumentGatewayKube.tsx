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
import { Danger } from 'design/Alert';
import { Flex, Text, ButtonPrimary } from 'design';

import * as types from 'teleterm/ui/services/workspacesService';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { useAsync, CanceledError } from 'shared/hooks/useAsync';
import Document from 'teleterm/ui/Document';
import { DocumentTerminal } from 'teleterm/ui/DocumentTerminal';
import { routing } from 'teleterm/ui/uri';

import { Reconnect } from './Reconnect';

export const DocumentGatewayKube = (props: {
  visible: boolean;
  doc: types.DocumentGatewayKube;
}) => {
  const { doc, visible } = props;
  const ctx = useAppContext();
  const { documentsService } = useWorkspaceContext();
  const { params } = routing.parseKubeUri(doc.targetUri);
  const [connectAttempt, createGateway] = useAsync(async () => {
    await retryWithRelogin(ctx, doc.targetUri, () =>
      ctx.clustersService.createGateway({
        targetUri: doc.targetUri,
        user: '',
      })
    );
  });

  useEffect(() => {
    documentsService.update(doc.uri, { status: 'connecting' });

    if (connectAttempt.status === '') {
      (async () => {
        const [, error] = await createGateway();
        if (!(error instanceof CanceledError)) {
          documentsService.update(doc.uri, { status: 'error' });
        }
      })();
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
