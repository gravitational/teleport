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
import TdpClient from 'teleport/lib/tdp/client';

export default {
  title: 'Teleport/DesktopSession',
};

export const Processing = () => (
  <DesktopSession
    {...props}
    attempt={{ status: 'processing' }}
    connection={{ status: 'connecting', statusText: 'Connecting...' }}
  />
);
export const Connecting = () => (
  <DesktopSession
    {...props}
    attempt={{ status: 'success' }}
    connection={{ status: 'connecting', statusText: 'Connecting...' }}
  />
);
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

const client = new TdpClient('wss://socketAddr.gov', 'username');
client.init = () => {};
client.connect = () => {};
client.resize = (w: number, h: number) => {};
client.disconnect = () => {};

const props: State = {
  tdpClient: client,
  username: 'user',
  hostname: 'host.com',
  attempt: { status: 'processing' },
  clipboard: false,
  recording: false,
  connection: { status: 'connecting', statusText: 'Connecting...' },
  setConnection: () => {},
};
