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
import { ComponentProps, useCallback, useEffect } from 'react';
import { z } from 'zod';

import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import Document from 'teleterm/ui/Document';
import { DocumentGateway } from 'teleterm/ui/services/workspacesService';

import { PortFieldInput } from '../components/FieldInputs';
import { FormFields, OfflineGateway } from '../components/OfflineGateway';
import { useGateway } from '../DocumentGateway/useGateway';
import { retryWithRelogin } from '../utils';
import { AppGateway } from './AppGateway';

export function DocumentGatewayApp(props: {
  doc: DocumentGateway;
  visible: boolean;
}) {
  const { doc, visible } = props;
  const appCtx = useAppContext();
  const { tshd } = appCtx;
  const {
    gateway,
    defaultPort,
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

  const isMultiPort = !!doc.targetSubresourceName;
  // TypeScript doesn't seem to be able to properly infer a simpler construct such as
  //
  //     isMultiPort ? multiPortSchema : singlePortSchema
  //
  // This code would always infer formSchema to be that of the simpler type (singlePortSchema), so
  // any errors in multiPortSchema would not be caught.
  let formSchema: ComponentProps<typeof OfflineGateway>['formSchema'] =
    singlePortSchema;
  if (isMultiPort) {
    formSchema = multiPortSchema;
  }

  const [tcpPortsAttempt, getTcpPorts] = useAsync(
    useCallback(
      () =>
        retryWithRelogin(appCtx, doc.targetUri, () =>
          tshd
            .getApp({ appUri: doc.targetUri })
            .then(({ response }) => response.app.tcpPorts)
        ),
      [appCtx, doc.targetUri, tshd]
    )
  );

  useEffect(() => {
    // Fetch TCP ports, but only when the gateway points at a multi-port TCP app and when the tab is
    // visible. This is so that if the user reopens a session with a lot of app gateways, we don't
    // fetch all ports at once.
    if (visible && isMultiPort && tcpPortsAttempt.status === '') {
      void getTcpPorts();
    }
  }, [visible, isMultiPort, tcpPortsAttempt, getTcpPorts]);

  return (
    <Document visible={props.visible}>
      {!connected ? (
        <OfflineGateway
          connectAttempt={connectAttempt}
          gatewayKind="app"
          targetName={doc.targetName}
          reconnect={reconnect}
          formSchema={formSchema}
          renderFormControls={(isProcessing: boolean) => (
            <>
              <PortFieldInput
                name={FormFields.LocalPort}
                label="Local Port (optional)"
                defaultValue={defaultPort}
                mb={0}
                readonly={isProcessing}
              />
              {isMultiPort && (
                <PortFieldInput
                  name={FormFields.TargetSubresourceName}
                  label="Target Port"
                  defaultValue={doc.targetSubresourceName}
                  required
                  mb={0}
                  readonly={isProcessing}
                />
              )}
            </>
          )}
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
          getTcpPorts={getTcpPorts}
          tcpPortsAttempt={tcpPortsAttempt}
        />
      )}
    </Document>
  );
}

const singlePortSchema = z.object({ [FormFields.LocalPort]: z.string() });
const multiPortSchema = singlePortSchema.extend({
  [FormFields.TargetSubresourceName]: z.string(),
});
