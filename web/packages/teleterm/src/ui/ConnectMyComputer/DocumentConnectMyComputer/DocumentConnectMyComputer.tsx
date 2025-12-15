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

import { useCallback } from 'react';

import Indicator from 'design/Indicator';

import Document from 'teleterm/ui/Document';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import * as types from 'teleterm/ui/services/workspacesService';

import { useConnectMyComputerContext } from '../connectMyComputerContext';
import { Setup } from './Setup';
import { Status } from './Status';

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
