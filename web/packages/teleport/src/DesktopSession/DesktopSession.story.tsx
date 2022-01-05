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
import TdpClient, {
  TdpClientEvent,
  ImageFragment,
} from 'teleport/lib/tdp/client';
import useAttempt from 'shared/hooks/useAttemptNext';
import { arrayBuf2260x1130 } from '../lib/tdp/fixtures';

export default {
  title: 'Teleport/DesktopSession',
};

const fakeClient = () => {
  const client = new TdpClient('wss://socketAddr.gov', 'username');
  client.init = () => {}; // Don't actually try to connect to a websocket.
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
  clipboard: false,
  recording: false,
  tdpClient: fakeClient(),
  username: 'user',
  onWsOpen: () => {},
  onWsClose: () => {},
  wsConnection: 'closed',
  disconnected: false,
  setDisconnected: () => null,
  onImageFragment: (ctx: CanvasRenderingContext2D, data: ImageFragment) => {},
  onTdpError: (err: Error) => {},
  onKeyDown: (cli: TdpClient, e: KeyboardEvent) => {},
  onKeyUp: (cli: TdpClient, e: KeyboardEvent) => {},
  onMouseMove: (cli: TdpClient, canvas: HTMLCanvasElement, e: MouseEvent) => {},
  onMouseDown: (cli: TdpClient, e: MouseEvent) => {},
  onMouseUp: (cli: TdpClient, e: MouseEvent) => {},
  onMouseWheelScroll: (cli: TdpClient, e: WheelEvent) => {},
};

export const Processing = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'processing' }}
    tdpConnection={{ status: 'processing' }}
    wsConnection={'open'}
    disconnected={false}
  />
);

export const ConnectedSettingsFalse = () => {
  const client = fakeClient();
  client.init = () => {
    client.emit(TdpClientEvent.IMAGE_FRAGMENT);
  };

  return (
    <DesktopSession
      {...props}
      tdpClient={client}
      fetchAttempt={{ status: 'success' }}
      tdpConnection={{ status: 'success' }}
      wsConnection={'open'}
      disconnected={false}
      clipboard={false}
      recording={false}
      onImageFragment={(ctx: CanvasRenderingContext2D, data: ImageFragment) => {
        fillGray(ctx.canvas);
      }}
    />
  );
};

export const ConnectedSettingsTrue = () => {
  const client = fakeClient();
  client.init = () => {
    client.emit(TdpClientEvent.IMAGE_FRAGMENT);
  };

  return (
    <DesktopSession
      {...props}
      tdpClient={client}
      fetchAttempt={{ status: 'success' }}
      tdpConnection={{ status: 'success' }}
      wsConnection={'open'}
      disconnected={false}
      clipboard={true}
      recording={true}
      onImageFragment={(ctx: CanvasRenderingContext2D, data: ImageFragment) => {
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
export const BothError = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'failed', statusText: 'some fetch  error' }}
    tdpConnection={{
      status: 'failed',
      statusText: 'some connection error',
    }}
  />
);

export const Performance = () => {
  const client = fakeClient();
  client.init = () => {
    for (let i = 0; i < arrayBuf2260x1130.length; i++) {
      client.processMessage(arrayBuf2260x1130[i]);
    }
  };
  var startTime,
    endTime,
    i = 0,
    resized = false,
    resize = (canvas: HTMLCanvasElement) => {
      // Hardcoded to match fixture
      const width = 2260;
      const height = 1130;

      // If it's resolution does not match change it
      if (canvas.width !== width || canvas.height !== height) {
        canvas.width = width;
        canvas.height = height;
      }
      resized = true;
    };

  return (
    <DesktopSession
      {...props}
      tdpClient={client}
      fetchAttempt={{ status: 'success' }}
      tdpConnection={{ status: 'success' }}
      wsConnection={'open'}
      disconnected={false}
      onImageFragment={(ctx: CanvasRenderingContext2D, data: ImageFragment) => {
        if (!resized) {
          resize(ctx.canvas);
        }
        if (i === 0) {
          startTime = performance.now();
        }

        ctx.drawImage(data.image, data.left, data.top);

        if (i === arrayBuf2260x1130.length - 1) {
          endTime = performance.now();
          // eslint-disable-next-line no-console
          console.log(`Total time (ms): ${endTime - startTime}`);
        }
        i++;
      }}
    />
  );
};
