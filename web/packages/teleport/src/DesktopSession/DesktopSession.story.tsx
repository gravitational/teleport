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

import React, { useState } from 'react';
import { ButtonPrimary } from 'design/Button';
import { NotificationItem } from 'shared/components/Notification';

import { TdpClient, TdpClientEvent } from 'teleport/lib/tdp';

import { State } from './useDesktopSession';
import { DesktopSession } from './DesktopSession';

export default {
  title: 'Teleport/DesktopSession',
};

const fakeClient = () => {
  const client = new TdpClient('wss://socketAddr.gov');
  client.connect = async () => {}; // Don't actually try to connect to a websocket.
  return client;
};

const fillGray = (canvas: HTMLCanvasElement) => {
  var ctx = canvas.getContext('2d');
  ctx.fillStyle = 'gray';
  ctx.fillRect(0, 0, canvas.width, canvas.height);
};

const props: State = {
  hostname: 'host.com',
  fetchAttempt: { status: 'processing' },
  tdpConnection: { status: 'processing' },
  clipboardSharingEnabled: false,
  tdpClient: fakeClient(),
  username: 'user',
  clientOnWsOpen: () => {},
  clientOnWsClose: () => {},
  wsConnection: 'closed',
  disconnected: false,
  setDisconnected: () => {},
  setClipboardSharingEnabled: () => {},
  directorySharingState: {
    canShare: true,
    isSharing: false,
  },
  setDirectorySharingState: () => {},
  onShareDirectory: () => {},
  clientOnPngFrame: () => {},
  clientOnBitmapFrame: () => {},
  clientOnClientScreenSpec: () => {},
  clientScreenSpecToRequest: { width: 0, height: 0 },
  clientOnTdpError: () => {},
  clientOnTdpWarning: () => {},
  canvasOnKeyDown: () => {},
  canvasOnKeyUp: () => {},
  canvasOnMouseMove: () => {},
  canvasOnMouseDown: () => {},
  canvasOnMouseUp: () => {},
  canvasOnMouseWheelScroll: () => {},
  canvasOnContextMenu: () => false,
  canvasOnFocusOut: () => {},
  clientOnClipboardData: async () => {},
  setTdpConnection: () => {},
  webauthn: {
    errorText: '',
    requested: false,
    authenticate: () => {},
    setState: () => {},
    addMfaToScpUrls: false,
  },
  isUsingChrome: true,
  showAnotherSessionActiveDialog: false,
  setShowAnotherSessionActiveDialog: () => {},
  warnings: [],
  onRemoveWarning: () => {},
};

export const Processing = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'processing' }}
    tdpConnection={{ status: 'processing' }}
    clipboardSharingEnabled={true}
    wsConnection={'open'}
    disconnected={false}
  />
);

export const TdpProcessing = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    tdpConnection={{ status: 'processing' }}
    clipboardSharingEnabled={true}
    wsConnection={'open'}
    disconnected={false}
  />
);

export const InvalidProcessingState = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'processing' }}
    tdpConnection={{ status: 'success' }}
    clipboardSharingEnabled={true}
    wsConnection={'open'}
    disconnected={false}
  />
);

export const ConnectedSettingsFalse = () => {
  const client = fakeClient();
  client.connect = async () => {
    client.emit(TdpClientEvent.TDP_PNG_FRAME);
  };

  return (
    <DesktopSession
      {...props}
      tdpClient={client}
      fetchAttempt={{ status: 'success' }}
      tdpConnection={{ status: 'success' }}
      wsConnection={'open'}
      disconnected={false}
      clipboardSharingEnabled={false}
      clientOnPngFrame={(ctx: CanvasRenderingContext2D) => {
        fillGray(ctx.canvas);
      }}
    />
  );
};

export const ConnectedSettingsTrue = () => {
  const client = fakeClient();
  client.connect = async () => {
    client.emit(TdpClientEvent.TDP_PNG_FRAME);
  };

  return (
    <DesktopSession
      {...props}
      tdpClient={client}
      fetchAttempt={{ status: 'success' }}
      tdpConnection={{ status: 'success' }}
      wsConnection={'open'}
      disconnected={false}
      clipboardSharingEnabled={true}
      directorySharingState={{
        canShare: true,
        isSharing: true,
      }}
      clientOnPngFrame={(ctx: CanvasRenderingContext2D) => {
        fillGray(ctx.canvas);
      }}
    />
  );
};

export const Disconnected = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    tdpConnection={{ status: 'success' }}
    wsConnection={'open'}
    disconnected={true}
  />
);

export const FetchError = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'failed', statusText: 'some fetch  error' }}
    tdpConnection={{ status: 'success' }}
    wsConnection={'open'}
    disconnected={false}
  />
);

export const ConnectionError = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    tdpConnection={{
      status: 'failed',
      statusText: 'some connection error',
    }}
    wsConnection={'closed'}
    disconnected={false}
  />
);

export const UnintendedDisconnect = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    tdpConnection={{ status: 'success' }}
    disconnected={false}
    wsConnection={'closed'}
  />
);

export const WebAuthnPrompt = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'processing' }}
    tdpConnection={{ status: 'processing' }}
    clipboardSharingEnabled={true}
    wsConnection={'open'}
    disconnected={false}
    webauthn={{
      errorText: '',
      requested: true,
      authenticate: () => {},
      setState: () => {},
      addMfaToScpUrls: false,
    }}
  />
);

export const AnotherSessionActive = () => (
  <DesktopSession {...props} showAnotherSessionActiveDialog={true} />
);

export const Warnings = () => {
  const client = fakeClient();
  client.connect = async () => {
    client.emit(TdpClientEvent.TDP_PNG_FRAME);
  };

  const [warnings, setWarnings] = useState<NotificationItem[]>([]);

  const addWarning = () => {
    setWarnings(prevItems => [
      ...prevItems,
      {
        id: crypto.randomUUID(),
        severity: 'warn',
        content:
          "Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s.",
      },
    ]);
  };

  const removeWarning = (id: string) => {
    setWarnings(prevState => prevState.filter(warning => warning.id !== id));
  };

  return (
    <>
      <ButtonPrimary onClick={addWarning} mb={1}>
        Add Warning
      </ButtonPrimary>
      <DesktopSession
        {...props}
        tdpClient={client}
        fetchAttempt={{ status: 'success' }}
        tdpConnection={{ status: 'success' }}
        wsConnection={'open'}
        disconnected={false}
        clipboardSharingEnabled={true}
        directorySharingState={{
          canShare: true,
          isSharing: true,
        }}
        clientOnPngFrame={(ctx: CanvasRenderingContext2D) => {
          fillGray(ctx.canvas);
        }}
        warnings={warnings}
        onRemoveWarning={removeWarning}
      />
    </>
  );
};
