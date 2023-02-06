/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { MemoryRouter } from 'react-router';

import { DownloadScript } from './DownloadScript';
import { State } from './useDownloadScript';

export default {
  title: 'Teleport/Discover/Server/DownloadScript',
};

export const Polling = () => (
  <MemoryRouter>
    <DownloadScript {...props} />
  </MemoryRouter>
);

export const PollingSuccess = () => (
  <MemoryRouter>
    <DownloadScript {...props} pollState="success" />
  </MemoryRouter>
);

export const PollingError = () => (
  <MemoryRouter>
    <DownloadScript {...props} pollState="error" />
  </MemoryRouter>
);

export const Processing = () => (
  <MemoryRouter>
    <DownloadScript {...props} attempt={{ status: 'processing' }} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <DownloadScript
      {...props}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  </MemoryRouter>
);

const props: State = {
  attempt: {
    status: 'success',
    statusText: '',
  },
  pollState: 'polling',
  nextStep: () => null,
  joinToken: {
    id: 'some-join-token-hash',
    expiryText: '4 hours',
    expiry: new Date(),
  },
  regenerateScriptAndRepoll: () => null,
  countdownTime: { minutes: 5, seconds: 0 },
};
