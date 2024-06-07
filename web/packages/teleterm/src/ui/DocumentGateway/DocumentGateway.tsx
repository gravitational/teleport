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

import React from 'react';

import Document from 'teleterm/ui/Document';
import * as types from 'teleterm/ui/services/workspacesService';

import { OfflineGateway } from '../components/OfflineGateway';

import { useDocumentGateway } from './useDocumentGateway';
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
      <DocumentGateway {...state} targetName={doc.targetName} />
    </Document>
  );
}

export type DocumentGatewayProps = ReturnType<typeof useDocumentGateway> & {
  targetName: string;
};

export function DocumentGateway(props: DocumentGatewayProps) {
  if (!props.connected) {
    return (
      <OfflineGateway
        connectAttempt={props.connectAttempt}
        reconnect={props.reconnect}
        gatewayPort={{ isSupported: true, defaultPort: props.defaultPort }}
        targetName={props.targetName}
        gatewayKind="database"
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
