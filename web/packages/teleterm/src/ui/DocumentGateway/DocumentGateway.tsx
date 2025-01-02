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

import Document from 'teleterm/ui/Document';
import * as types from 'teleterm/ui/services/workspacesService';
import { getCliCommandArgv0 } from 'teleterm/services/tshd/gateway';

import { OfflineGateway } from '../components/OfflineGateway';
import { useWorkspaceContext } from '../Documents';

import { useGateway } from './useGateway';
import { OnlineDocumentGateway } from './OnlineDocumentGateway';

export function DocumentGateway(props: {
  visible: boolean;
  doc: types.DocumentGateway;
}) {
  const { doc, visible } = props;
  const { documentsService } = useWorkspaceContext();

  const {
    connected,
    // Needed for OfflineGateway.
    connectAttempt,
    reconnect,
    defaultPort,
    // Needed for OnlineDocumentGateway.
    gateway,
    disconnect,
    disconnectAttempt,
    changePort,
    changePortAttempt,
    changeTargetSubresourceNameAttempt: changeDbNameAttempt,
    changeTargetSubresourceName: changeDbName,
  } = useGateway(doc);

  const runCliCommand = () => {
    const command = getCliCommandArgv0(gateway.gatewayCliCommand);
    const title = `${command} · ${doc.targetUser}@${doc.targetName}`;

    const cliDoc = documentsService.createGatewayCliDocument({
      title,
      targetUri: doc.targetUri,
      targetUser: doc.targetUser,
      targetName: doc.targetName,
      targetProtocol: gateway.protocol,
    });
    documentsService.add(cliDoc);
    documentsService.setLocation(cliDoc.uri);
  };

  if (!connected) {
    return (
      <Document visible={visible}>
        <OfflineGateway
          connectAttempt={connectAttempt}
          reconnect={reconnect}
          gatewayPort={{ isSupported: true, defaultPort }}
          targetName={doc.targetName}
          gatewayKind="database"
        />
      </Document>
    );
  }

  return (
    <Document visible={visible}>
      <OnlineDocumentGateway
        disconnect={disconnect}
        disconnectAttempt={disconnectAttempt}
        changeDbName={changeDbName}
        changeDbNameAttempt={changeDbNameAttempt}
        changePortAttempt={changePortAttempt}
        gateway={gateway}
        changePort={changePort}
        runCliCommand={runCliCommand}
      />
    </Document>
  );
}
