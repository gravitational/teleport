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

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { DocumentGateway } from 'teleterm/ui/services/workspacesService';
import Document from 'teleterm/ui/Document';

import { useDocumentGateway } from '../DocumentGateway/useDocumentGateway';

import { OfflineGateway } from '../components/OfflineGateway';

import { AppGateway } from './AppGateway';

export function DocumentGatewayApp(props: {
  doc: DocumentGateway;
  visible: boolean;
}) {
  const ctx = useAppContext();
  const {
    gateway,
    changePort,
    changePortAttempt,
    connected,
    connectAttempt,
    disconnect,
    disconnectAttempt,
    reconnect,
  } = useDocumentGateway(props.doc);

  ctx.clustersService.useState();

  return (
    <Document visible={props.visible}>
      {!connected ? (
        <OfflineGateway
          connectAttempt={connectAttempt}
          gatewayKind="app"
          targetName={props.doc.targetName}
          gatewayPort={{ isSupported: true, defaultPort: props.doc.port }}
          reconnect={reconnect}
        />
      ) : (
        <AppGateway
          gateway={gateway}
          disconnect={disconnect}
          disconnectAttempt={disconnectAttempt}
          changePort={changePort}
          changePortAttempt={changePortAttempt}
        />
      )}
    </Document>
  );
}
