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

import React from 'react';

import Indicator from 'design/Indicator';

import * as types from 'teleterm/ui/services/workspacesService';
import Document from 'teleterm/ui/Document';

import { useConnectMyComputerContext } from './connectMyComputerContext';
import { DocumentConnectMyComputerStatus } from './DocumentConnectMyComputerStatus/DocumentConnectMyComputerStatus';
import { DocumentConnectMyComputerSetup } from './DocumentConnectMyComputerSetup/DocumentConnectMyComputerSetup';

interface DocumentConnectMyComputerProps {
  visible: boolean;
  doc: types.DocumentConnectMyComputer;
}

export function DocumentConnectMyComputer(
  props: DocumentConnectMyComputerProps
) {
  const { isAgentConfiguredAttempt } = useConnectMyComputerContext();
  const shouldShowSetup =
    isAgentConfiguredAttempt.status === 'success' &&
    !isAgentConfiguredAttempt.data;

  if (isAgentConfiguredAttempt.status === 'processing') {
    return <Indicator m="auto" />;
  }

  return (
    <Document visible={props.visible}>
      {shouldShowSetup ? (
        <DocumentConnectMyComputerSetup />
      ) : (
        <DocumentConnectMyComputerStatus />
      )}
    </Document>
  );
}
