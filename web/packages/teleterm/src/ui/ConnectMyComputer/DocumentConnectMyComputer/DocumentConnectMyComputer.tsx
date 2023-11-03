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

import React, { useCallback } from 'react';

import Indicator from 'design/Indicator';

import * as types from 'teleterm/ui/services/workspacesService';
import Document from 'teleterm/ui/Document';
import { useWorkspaceContext } from 'teleterm/ui/Documents';

import { useConnectMyComputerContext } from '../connectMyComputerContext';

import { Status } from './Status';
import { Setup } from './Setup';

export function DocumentConnectMyComputer(props: {
  visible: boolean;
  doc: types.DocumentConnectMyComputer;
}) {
  const { documentsService } = useWorkspaceContext();
  const { isAgentConfiguredAttempt } = useConnectMyComputerContext();
  const shouldShowSetup =
    isAgentConfiguredAttempt.status === 'success' &&
    !isAgentConfiguredAttempt.data;

  const closeDocument = useCallback(() => {
    documentsService.close(props.doc.uri);
  }, [documentsService, props.doc.uri]);

  const updateDocumentStatus = useCallback(
    (status: types.DocumentConnectMyComputer['status']) => {
      documentsService.update(props.doc.uri, { status });
    },
    [documentsService, props.doc.uri]
  );

  if (isAgentConfiguredAttempt.status === 'processing') {
    return <Indicator m="auto" />;
  }

  return (
    <Document visible={props.visible}>
      {shouldShowSetup ? (
        <Setup updateDocumentStatus={updateDocumentStatus} />
      ) : (
        <Status closeDocument={closeDocument} />
      )}
    </Document>
  );
}
