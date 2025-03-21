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

import { useEffect, useState } from 'react';

import { ButtonPrimary } from 'design/Button';
import { NotificationItem } from 'shared/components/Notification';

import { TdpClient, TdpClientEvent } from 'teleport/lib/tdp';
import { BitmapFrame } from 'teleport/lib/tdp/client';
import { makeDefaultMfaState } from 'teleport/lib/useMfa';

import { DesktopSession } from './DesktopSession';
import { State } from './useDesktopSession';

export default {
  title: 'Teleport/DesktopSession',
};

const fakeClient = () => {
  const client = new TdpClient('wss://socketAddr.gov');
  // Don't try to connect to a websocket.
  client.connect = async () => {
    emitGrayFrame(client);
  };
  return client;
};

const props: State = {
  hostname: 'host.com',
  fetchAttempt: { status: 'processing' },
  clipboardSharingState: {
    allowedByAcl: false,
    browserSupported: false,
  },
  tdpClient: fakeClient(),
  username: 'user',
  setClipboardSharingState: () => {},
  directorySharingState: {
    allowedByAcl: true,
    browserSupported: true,
    directorySelected: false,
  },
  sendLocalClipboardToRemote: async () => {},
  onClipboardData: async () => {},
  addAlert: () => {},
  setDirectorySharingState: () => {},
  onShareDirectory: () => {},
  clientScreenSpecToRequest: { width: 0, height: 0 },
  mfa: makeDefaultMfaState(),
  showAnotherSessionActiveDialog: false,
  setShowAnotherSessionActiveDialog: () => {},
  alerts: [],
  onRemoveAlert: () => {},
};

export const Processing = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'processing' }}
    clipboardSharingState={{
      allowedByAcl: false,
      browserSupported: false,
      readState: 'granted',
      writeState: 'granted',
    }}
  />
);

export const FetchError = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'failed', statusText: 'some fetch  error' }}
  />
);

export const TdpError = () => {
  useEffect(() => {
    props.tdpClient.emit(TdpClientEvent.TDP_ERROR, new Error('some tdp error'));
  }, []);

  return <DesktopSession {...props} fetchAttempt={{ status: 'success' }} />;
};

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

export const ConnectedSettingsTrue = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
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

export const Disconnected = () => {
  useEffect(() => {
    props.tdpClient.emit(TdpClientEvent.WS_CLOSE, 'session disconnected');
  }, []);

  return <DesktopSession {...props} fetchAttempt={{ status: 'success' }} />;
};

export const WebAuthnPrompt = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'processing' }}
    clipboardSharingState={{
      allowedByAcl: false,
      browserSupported: false,
      readState: 'granted',
      writeState: 'granted',
    }}
    mfa={{
      ...makeDefaultMfaState(),
      attempt: {
        status: 'processing',
        statusText: '',
        data: null,
      },
      challenge: {
        webauthnPublicKey: {
          challenge: new ArrayBuffer(1),
        },
      },
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
    clipboardSharingState={{ browserSupported: true, allowedByAcl: false }}
  />
);

export const ClipboardSharingDisabledIncompatibleBrowser = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    clipboardSharingState={{ browserSupported: false, allowedByAcl: true }}
  />
);

export const ClipboardSharingDisabledBrowserPermissions = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    clipboardSharingState={{
      browserSupported: true,
      allowedByAcl: true,
      readState: 'granted',
      writeState: 'denied',
    }}
  />
);

export const Alerts = () => {
  const client = fakeClient();
  client.connect = async () => {
    emitGrayFrame(client);
  };

  const [alerts, setAlerts] = useState<NotificationItem[]>([]);

  const addWarning = () => {
    setAlerts(prevItems => [
      ...prevItems,
      {
        id: crypto.randomUUID(),
        severity: 'warn',
        content: `This is a warning message at ${new Date().toLocaleTimeString()}`,
      },
    ]);
  };

  const addInfo = () => {
    setAlerts(prevItems => [
      ...prevItems,
      {
        id: crypto.randomUUID(),
        severity: 'info',
        content: `This is an info message at ${new Date().toLocaleTimeString()}`,
      },
    ]);
  };

  const removeAlert = (id: string) => {
    setAlerts(prevState => prevState.filter(warning => warning.id !== id));
  };

  return (
    <>
      <ButtonPrimary onClick={addWarning} mb={1} mr={1}>
        Add Warning
      </ButtonPrimary>
      <ButtonPrimary onClick={addInfo} mb={1}>
        Add Info
      </ButtonPrimary>
      <DesktopSession
        {...props}
        tdpClient={client}
        fetchAttempt={{ status: 'success' }}
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
        alerts={alerts}
        onRemoveAlert={removeAlert}
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
