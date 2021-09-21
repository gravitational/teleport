/*
Copyright 2021 Gravitational, Inc.

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

import React, { useState } from 'react';
import { DesktopSession } from './DesktopSession';
import { State } from './useDesktopSession';
import TdpClient, { RenderData } from 'teleport/lib/tdp/client';
import { TdpClientConnectionState } from './useTdpClientCanvas';
import useAttempt from 'shared/hooks/useAttemptNext';

export default {
  title: 'Teleport/DesktopSession',
};

const client = new TdpClient('wss://socketAddr.gov', 'username');
client.init = () => {
  client.emit('init');
};
client.connect = () => {
  client.emit('connect');
};
client.resize = (w: number, h: number) => {};
client.disconnect = () => {
  client.emit('disconnect');
};

const fillGray = (canvas: HTMLCanvasElement) => {
  var ctx = canvas.getContext('2d');
  ctx.fillStyle = 'gray';
  ctx.fillRect(0, 0, canvas.width, canvas.height);
};

const props: State = {
  hostname: 'host.com',
  attempt: { status: 'processing' },
  clipboard: false,
  recording: false,
  tdpClient: client,
  connection: { status: 'connecting', statusText: 'Connecting...' },
  username: 'user',
  onInit: (cli: TdpClient, canvas: HTMLCanvasElement) => {
    fillGray(canvas);
  },
  onConnect: () => {},
  onRender: (canvas: HTMLCanvasElement, data: RenderData) => {},
  onDisconnect: () => {},
  onError: (err: Error) => {},
  onKeyDown: (cli: TdpClient, e: KeyboardEvent) => {},
  onKeyUp: (cli: TdpClient, e: KeyboardEvent) => {},
  onMouseMove: (cli: TdpClient, canvas: HTMLCanvasElement, e: MouseEvent) => {},
  onMouseDown: (cli: TdpClient, e: MouseEvent) => {},
  onMouseUp: (cli: TdpClient, e: MouseEvent) => {},
  onResize: (cli: TdpClient, canvas: HTMLCanvasElement) => {},
};

export const Processing = () => (
  <DesktopSession
    {...props}
    attempt={{ status: 'processing' }}
    connection={{ status: 'connecting' }}
  />
);

export const ProcessingToConnectingToDisplay = () => {
  const { attempt, setAttempt } = useAttempt('processing');
  const [connection, setConnection] = useState<TdpClientConnectionState>({
    status: 'connecting',
  });

  setTimeout(() => {
    setAttempt({ status: 'success' });
    setTimeout(() => {
      setConnection({ status: 'connected' });
    }, 1000);
  }, 1000);

  return (
    <DesktopSession {...props} attempt={attempt} connection={connection} />
  );
};
export const ConnectedSettingsFalse = () => (
  <DesktopSession
    {...props}
    attempt={{ status: 'success' }}
    connection={{ status: 'connected' }}
    clipboard={false}
    recording={false}
  />
);
export const ConnectedSettingsTrue = () => (
  <DesktopSession
    {...props}
    attempt={{ status: 'success' }}
    connection={{ status: 'connected' }}
    clipboard={true}
    recording={true}
  />
);
export const Disconnected = () => (
  <DesktopSession
    {...props}
    attempt={{ status: 'success' }}
    connection={{
      status: 'disconnected',
    }}
  />
);
export const Error = () => (
  <DesktopSession
    {...props}
    attempt={{ status: 'failed', statusText: 'some error message' }}
  />
);
