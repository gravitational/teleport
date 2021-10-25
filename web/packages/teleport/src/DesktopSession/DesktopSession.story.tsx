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

import React from 'react';
import { DesktopSession } from './DesktopSession';
import { State } from './useDesktopSession';
import TdpClient, { ImageData } from 'teleport/lib/tdp/client';
import useAttempt from 'shared/hooks/useAttemptNext';
import { arrayBuf2260x1130 } from '../lib/tdp/fixtures';

export default {
  title: 'Teleport/DesktopSession',
};

const fakeClient = () => {
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
  connectionAttempt: { status: 'processing' },
  clipboard: false,
  recording: false,
  tdpClient: fakeClient(),
  username: 'user',
  onInit: (cli: TdpClient, canvas: HTMLCanvasElement) => {
    fillGray(canvas);
  },
  onConnect: () => {},
  onRender: (ctx: CanvasRenderingContext2D, data: ImageData) => {},
  onDisconnect: () => {},
  onError: (err: Error) => {},
  onKeyDown: (cli: TdpClient, e: KeyboardEvent) => {},
  onKeyUp: (cli: TdpClient, e: KeyboardEvent) => {},
  onMouseMove: (cli: TdpClient, canvas: HTMLCanvasElement, e: MouseEvent) => {},
  onMouseDown: (cli: TdpClient, e: MouseEvent) => {},
  onMouseUp: (cli: TdpClient, e: MouseEvent) => {},
};

export const Processing = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'processing' }}
    connectionAttempt={{ status: 'processing' }}
  />
);

export const ProcessingToConnectingToDisplay = () => {
  const { attempt: fetchAttempt, setAttempt: setFetchAttempt } = useAttempt(
    'processing'
  );
  const { attempt: connection, setAttempt: setConnection } = useAttempt(
    'processing'
  );

  setTimeout(() => {
    setFetchAttempt({ status: 'success' });
    setTimeout(() => {
      setConnection({ status: 'success' });
    }, 1000);
  }, 1000);

  return (
    <DesktopSession
      {...props}
      fetchAttempt={fetchAttempt}
      connectionAttempt={connection}
    />
  );
};
export const ConnectedSettingsFalse = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    connectionAttempt={{ status: 'success' }}
    clipboard={false}
    recording={false}
  />
);
export const ConnectedSettingsTrue = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    connectionAttempt={{ status: 'success' }}
    clipboard={true}
    recording={true}
  />
);
export const Disconnected = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    connectionAttempt={{ status: '' }}
  />
);
export const FetchError = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'failed', statusText: 'some fetch  error' }}
    connectionAttempt={{ status: 'success' }}
  />
);
export const ConnectionError = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'success' }}
    connectionAttempt={{
      status: 'failed',
      statusText: 'some connection error',
    }}
  />
);
export const Performance = () => {
  const client = fakeClient();
  var startTime,
    endTime,
    i = 0;

  return (
    <DesktopSession
      {...props}
      fetchAttempt={{ status: 'success' }}
      connectionAttempt={{ status: 'success' }}
      tdpClient={client}
      onInit={(cli: TdpClient, canvas: HTMLCanvasElement) => {
        // Hardcoded to match fixture
        const width = 2260;
        const height = 1130;

        // If it's resolution does not match change it
        if (canvas.width !== width || canvas.height !== height) {
          canvas.width = width;
          canvas.height = height;
        }
        cli.emit('connect');
      }}
      onConnect={() => {
        for (let i = 0; i < arrayBuf2260x1130.length; i++) {
          client.processMessage(arrayBuf2260x1130[i]);
        }
      }}
      onRender={(ctx: CanvasRenderingContext2D, data: ImageData) => {
        ctx.drawImage(data.image, data.left, data.top);
        if (i === 0) {
          startTime = performance.now();
        } else if (i === arrayBuf2260x1130.length - 1) {
          endTime = performance.now();
          // eslint-disable-next-line no-console
          console.log(`Total time (ms): ${endTime - startTime}`);
        }
        i++;
      }}
    />
  );
};
