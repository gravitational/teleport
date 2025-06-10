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

import { Meta } from '@storybook/react';

import DialogConfirmation from 'design/DialogConfirmation';
import {
  makeErrorAttempt,
  makeProcessingAttempt,
  makeSuccessAttempt,
} from 'shared/hooks/useAsync';
import {
  BitmapFrame,
  BrowserFileSystem,
  ClientScreenSpec,
  TdpClient,
  TdpClientEvent,
} from 'shared/libs/tdp';
import { TdpError as RemoteTdpError } from 'shared/libs/tdp/client';

import { DesktopSession, DesktopSessionProps } from './DesktopSession';

const meta: Meta = {
  title: 'Shared/DesktopSession',
  decorators: [
    Story => (
      <div
        css={`
          position: relative;
          height: 90vh;
          width: 100%;
        `}
      >
        <Story />
      </div>
    ),
  ],
};

export default meta;

const fakeClient = () => {
  const client = new TdpClient(() => null, new BrowserFileSystem());
  // Don't try to connect to a websocket.
  client.connect = async options => {
    emitFrame(client, options.screenSpec);
  };
  return client;
};

const props: DesktopSessionProps = {
  aclAttempt: makeSuccessAttempt({
    clipboardSharingEnabled: true,
    directorySharingEnabled: true,
  }),
  client: fakeClient(),
  username: 'user',
  desktop: 'windows-11',
  browserSupportsSharing: true,
  hasAnotherSession: () => Promise.resolve(false),
};

export const Processing = () => (
  <DesktopSession {...props} aclAttempt={makeProcessingAttempt()} />
);

export const FetchError = () => (
  <DesktopSession
    {...props}
    aclAttempt={makeErrorAttempt(new Error('Network Error'))}
  />
);

export const TdpError = () => {
  const client = fakeClient();
  client.connect = async () => {
    client.emit(
      TdpClientEvent.ERROR,
      new RemoteTdpError(
        'RDP client exited with an error: Connection Timed Out.\n\n' +
          'Teleport could not connect to the host within the timeout period. This may be due to a firewall blocking the connection, an overloaded system, or network congestion.\n\n' +
          'To resolve this issue, ensure that the Teleport agent can reach the Windows host.\n\n' +
          'You can use the command "nc -vz HOST 3389" to help diagnose connectivity problems.'
      )
    );
  };

  return <DesktopSession {...props} client={client} />;
};

export const Connected = () => {
  return <DesktopSession {...props} />;
};

export const DisconnectedWithNoMessage = () => {
  const client = fakeClient();
  client.connect = async () => {
    client.emit(TdpClientEvent.TRANSPORT_CLOSE, undefined);
  };

  return <DesktopSession {...props} client={client} />;
};

export const MfaPrompt = () => (
  <DesktopSession
    {...props}
    customConnectionState={() => {
      return (
        <DialogConfirmation open={true} dialogCss={() => ({ maxWidth: 300 })}>
          This is a custom connection state, needed for per-session MFA. Web UI
          displays its standard AuthnDialog here. Connect utilizes the
          modalsService for per-session MFA, and only errors will be shown in
          this state.
        </DialogConfirmation>
      );
    }}
  />
);

export const AnotherSessionActive = () => (
  <DesktopSession {...props} hasAnotherSession={() => Promise.resolve(true)} />
);

export const SharingDisabledRbac = () => (
  <DesktopSession
    {...props}
    aclAttempt={makeSuccessAttempt({
      clipboardSharingEnabled: false,
      directorySharingEnabled: false,
    })}
  />
);

//TODO(gzdunek): Enable these stories after refactoring clipboard and directory sharing.
// Currently, the logic is hardcoded in the component so we have no control over it.
//
// export const ClipboardSharingDisabledIncompatibleBrowser = () => (
//   <DesktopSession
//     {...props}
//     fetchAttempt={{ status: 'success' }}
//     clipboardSharingState={{ browserSupported: false, allowedByAcl: true }}
//   />
// );
//
// export const ClipboardSharingDisabledBrowserPermissions = () => (
//   <DesktopSession
//     {...props}
//     fetchAttempt={{ status: 'success' }}
//     clipboardSharingState={{
//       browserSupported: true,
//       allowedByAcl: true,
//       readState: 'granted',
//       writeState: 'denied',
//     }}
//   />
// );

export const Alerts = () => {
  const client = fakeClient();
  client.connect = async options => {
    emitFrame(client, options.screenSpec);
    client.emit(
      TdpClientEvent.TDP_WARNING,
      'Potential performance issues detected. Expect possible lag or instability.'
    );
    client.emit(
      TdpClientEvent.TDP_INFO,
      'Connection initiated. Monitoring for potential issues.'
    );
  };

  return <DesktopSession {...props} client={client} />;
};

function emitFrame(client: TdpClient, spec: ClientScreenSpec) {
  const width = spec.width;
  const height = spec.height;
  const imageData = new ImageData(width, height);

  // Displays a nice gradient.
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const index = (y * width + x) * 4;

      // Calculate the gradients for each channel
      const r = Math.sin((x / width) * Math.PI) * 255; // Sinusoidal Red gradient
      const g = (y / height) * 255; // Vertical gradient for Green
      const b = Math.cos((x / width) * Math.PI) * 255; // Cosine-based Blue gradient

      imageData.data[index] = r; // Red
      imageData.data[index + 1] = g; // Green
      imageData.data[index + 2] = b; // Blue
      imageData.data[index + 3] = 255; // Alpha (fully opaque)
    }
  }

  const frame: BitmapFrame = {
    left: 0,
    top: 0,
    image_data: imageData,
  };

  client.emit(TdpClientEvent.TDP_CLIENT_SCREEN_SPEC, spec);
  client.emit(TdpClientEvent.TDP_BMP_FRAME, frame);
}
