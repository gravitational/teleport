/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import { DocumentGateway } from 'teleterm/ui/services/workspacesService';

import { OfflineGateway } from '../components/OfflineGateway';
import { useGateway } from '../DocumentGateway/useGateway';
import { AppGateway } from './AppGateway';

export function DocumentGatewayApp(props: {
  doc: DocumentGateway;
  visible: boolean;
}) {
  const { doc } = props;
  const {
    gateway,
    changePort: changeLocalPort,
    changePortAttempt: changeLocalPortAttempt,
    connected,
    connectAttempt,
    disconnect,
    disconnectAttempt,
    reconnect,
    changeTargetSubresourceName: changeTargetPort,
    changeTargetSubresourceNameAttempt: changeTargetPortAttempt,
  } = useGateway(doc);

  return (
    <Document visible={props.visible}>
      {!connected ? (
        <OfflineGateway
          connectAttempt={connectAttempt}
          gatewayKind="app"
          targetName={doc.targetName}
          gatewayPort={{ isSupported: true, defaultPort: doc.port }}
          reconnect={reconnect}
          portFieldLabel="Local Port (optional)"
        />
      ) : (
        <AppGateway
          gateway={gateway}
          disconnect={disconnect}
          disconnectAttempt={disconnectAttempt}
          changeLocalPort={changeLocalPort}
          changeLocalPortAttempt={changeLocalPortAttempt}
          changeTargetPort={changeTargetPort}
          changeTargetPortAttempt={changeTargetPortAttempt}
        />
      )}
    </Document>
  );
}
