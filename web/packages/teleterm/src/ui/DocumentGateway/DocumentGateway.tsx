/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';

import Document from 'teleterm/ui/Document';
import * as types from 'teleterm/ui/services/workspacesService';

import useDocumentGateway from './useDocumentGateway';
import { OfflineDocumentGateway } from './OfflineDocumentGateway';
import { OnlineDocumentGateway } from './OnlineDocumentGateway';

type Props = {
  visible: boolean;
  doc: types.DocumentGateway;
};

export default function Container(props: Props) {
  const { doc, visible } = props;
  const state = useDocumentGateway(doc);
  return (
    <Document visible={visible}>
      <DocumentGateway {...state} />
    </Document>
  );
}

export type DocumentGatewayProps = ReturnType<typeof useDocumentGateway>;

export function DocumentGateway(props: DocumentGatewayProps) {
  if (!props.connected) {
    return (
      <OfflineDocumentGateway
        connectAttempt={props.connectAttempt}
        reconnect={props.reconnect}
        defaultPort={props.defaultPort}
      />
    );
  }

  return (
    <OnlineDocumentGateway
      disconnect={props.disconnect}
      changeDbName={props.changeDbName}
      changeDbNameAttempt={props.changeDbNameAttempt}
      changePortAttempt={props.changePortAttempt}
      gateway={props.gateway}
      changePort={props.changePort}
      runCliCommand={props.runCliCommand}
    />
  );
}
