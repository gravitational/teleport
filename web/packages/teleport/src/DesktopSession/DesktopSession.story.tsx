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
import { BitmapFrame } from 'teleport/lib/tdp/client';

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

const props: State = {
  hostname: 'host.com',
  fetchAttempt: { status: 'processing' },
  tdpConnection: { status: 'processing' },
  clipboardSharingState: {
    allowedByAcl: false,
    browserSupported: false,
  },
  tdpClient: fakeClient(),
  username: 'user',
  wsConnection: { status: 'closed', statusText: 'websocket closed' },
  setClipboardSharingState: () => {},
  directorySharingState: {
    allowedByAcl: true,
    browserSupported: true,
    directorySelected: false,
  },
  addAlert: () => {},
  setWsConnection: () => {},
  setDirectorySharingState: () => {},
  onShareDirectory: () => {},
  onCtrlAltDel: () => {},
  setInitialTdpConnectionSucceeded: () => {},
  clientScreenSpecToRequest: { width: 0, height: 0 },
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
  showAnotherSessionActiveDialog: false,
  setShowAnotherSessionActiveDialog: () => {},
  warnings: [],
  onRemoveWarning: () => {},
  onResize: () => {},
};

export const BothProcessing = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'processing' }}
    tdpConnection={{ status: 'processing' }}
    clipboardSharingState={{
      allowedByAcl: false,
      browserSupported: false,
      readState: 'granted',
      writeState: 'granted',
    }}
    wsConnection={{ status: 'open' }}
  />
);

export const TdpProcessing = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    tdpConnection={{ status: 'processing' }}
    clipboardSharingState={{
      allowedByAcl: false,
      browserSupported: false,
      readState: 'granted',
      writeState: 'granted',
    }}
    wsConnection={{ status: 'open' }}
  />
);

export const FetchProcessing = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'processing' }}
    tdpConnection={{ status: 'success' }}
    clipboardSharingState={{
      allowedByAcl: false,
      browserSupported: false,
      readState: 'granted',
      writeState: 'granted',
    }}
    wsConnection={{ status: 'open' }}
  />
);

export const FetchError = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'failed', statusText: 'some fetch  error' }}
    tdpConnection={{ status: 'success' }}
    wsConnection={{ status: 'open' }}
  />
);

export const TdpError = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    tdpConnection={{
      status: 'failed',
      statusText: 'some tdp error',
    }}
    wsConnection={{ status: 'closed' }}
  />
);

export const TdpGraceful = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    tdpConnection={{
      status: '',
      statusText: 'some tdp message',
    }}
    wsConnection={{ status: 'closed' }}
  />
);

export const ConnectedSettingsFalse = () => {
  const client = fakeClient();
  client.connect = async () => {
    emitGrayFrame(client);
  };

  return (
    <DesktopSession
      {...props}
      tdpClient={client}
      fetchAttempt={{ status: 'success' }}
      tdpConnection={{ status: 'success' }}
      wsConnection={{ status: 'open' }}
      clipboardSharingState={{
        allowedByAcl: false,
        browserSupported: false,
        readState: 'denied',
        writeState: 'denied',
      }}
      directorySharingState={{
        allowedByAcl: false,
        browserSupported: false,
        directorySelected: false,
      }}
    />
  );
};

export const ConnectedSettingsTrue = () => {
  const client = fakeClient();
  client.connect = async () => {
    emitGrayFrame(client);
  };

  return (
    <DesktopSession
      {...props}
      tdpClient={client}
      fetchAttempt={{ status: 'success' }}
      tdpConnection={{ status: 'success' }}
      wsConnection={{ status: 'open' }}
      clipboardSharingState={{
        allowedByAcl: true,
        browserSupported: true,
        readState: 'granted',
        writeState: 'granted',
      }}
      directorySharingState={{
        allowedByAcl: true,
        browserSupported: true,
        directorySelected: true,
      }}
    />
  );
};

export const Disconnected = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    tdpConnection={{ status: 'success' }}
    wsConnection={{ status: 'closed', statusText: 'session disconnected' }}
  />
);

export const UnintendedDisconnect = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    tdpConnection={{ status: 'success' }}
    wsConnection={{ status: 'closed' }}
  />
);

export const WebAuthnPrompt = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'processing' }}
    tdpConnection={{ status: 'processing' }}
    clipboardSharingState={{
      allowedByAcl: false,
      browserSupported: false,
      readState: 'granted',
      writeState: 'granted',
    }}
    wsConnection={{ status: 'open' }}
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

export const ClipboardSharingDisabledRbac = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    tdpConnection={{ status: 'success' }}
    wsConnection={{ status: 'open' }}
    clipboardSharingState={{ browserSupported: true, allowedByAcl: false }}
  />
);

export const ClipboardSharingDisabledIncompatibleBrowser = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    tdpConnection={{ status: 'success' }}
    wsConnection={{ status: 'open' }}
    clipboardSharingState={{ browserSupported: false, allowedByAcl: true }}
  />
);

export const ClipboardSharingDisabledBrowserPermissions = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    tdpConnection={{ status: 'success' }}
    wsConnection={{ status: 'open' }}
    clipboardSharingState={{
      browserSupported: true,
      allowedByAcl: true,
      readState: 'granted',
      writeState: 'denied',
    }}
  />
);

export const Warnings = () => {
  const client = fakeClient();
  client.connect = async () => {
    emitGrayFrame(client);
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
        wsConnection={{ status: 'open' }}
        clipboardSharingState={{
          allowedByAcl: true,
          browserSupported: true,
          readState: 'granted',
          writeState: 'granted',
        }}
        directorySharingState={{
          allowedByAcl: true,
          browserSupported: true,
          directorySelected: true,
        }}
        warnings={warnings}
        onRemoveWarning={removeWarning}
      />
    </>
  );
};

function emitGrayFrame(client: TdpClient) {
  const width = 300;
  const height = 100;
  const imageData = new ImageData(width, height);

  // Fill with gray (RGB: 128, 128, 128)
  for (let i = 0; i < imageData.data.length; i += 4) {
    imageData.data[i] = 128; // Red
    imageData.data[i + 1] = 128; // Green
    imageData.data[i + 2] = 128; // Blue
    imageData.data[i + 3] = 255; // Alpha (fully opaque)
  }

  const frame: BitmapFrame = {
    left: 0,
    top: 0,
    image_data: imageData,
  };

  client.emit(TdpClientEvent.TDP_BMP_FRAME, frame);
}
