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

import React, { useState } from 'react';

import Document from 'teleterm/ui/Document';
import * as types from 'teleterm/ui/services/workspacesService';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { DocumentTerminal } from 'teleterm/ui/DocumentTerminal';

/**
 * DocumentGatewayCliClient creates a terminal session that targets the given gateway.
 *
 * It waits for the gateway to be created before starting the terminal session. This is because when
 * the user restarts the app, DocumentGateway and DocumentGatewayCliClient are restored at the same
 * time and it takes a sec to actually start the gateway.
 */
export const DocumentGatewayCliClient = (props: {
  visible: boolean;
  doc: types.DocumentGatewayCliClient;
}) => {
  const { clustersService } = useAppContext();
  clustersService.useState();

  const { doc, visible } = props;
  const [hasRenderedTerminal, setHasRenderedTerminal] = useState(false);

  const gateway = clustersService.findGatewayByConnectionParams(
    doc.targetUri,
    doc.targetUser
  );

  // Once we render the terminal even once, we want to keep it visible. Otherwise removing the
  // gateway would mean that this document would immediately close the PTY.
  //
  // After the gateway is closed, the CLI client will not be able to interact with the gateway
  // target, but the user might still want to inspect the output.
  if (gateway || hasRenderedTerminal) {
    if (!hasRenderedTerminal) {
      setHasRenderedTerminal(true);
    }

    return <DocumentTerminal doc={doc} visible={visible} />;
  }

  return <Document visible={visible}>There's no gateway</Document>;
};
