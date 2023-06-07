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

import { TestConnection } from './TestConnection';

import type { State } from './useTestConnection';

export default {
  title: 'Teleport/Discover/Shared/ConnectionDiagnostic/Server',
};

export const Init = () => (
  <MemoryRouter>
    <TestConnection {...props} />
  </MemoryRouter>
);

const props: State = {
  attempt: {
    status: 'success',
    statusText: '',
  },
  logins: ['root', 'llama', 'george_washington_really_long_name_testing'],
  startSshSession: () => null,
  testConnection: () => null,
  nextStep: () => null,
  prevStep: () => null,
  diagnosis: null,
  canTestConnection: true,
  username: 'teleport-username',
  authType: 'local',
  clusterId: 'some-cluster-id',
  showMfaDialog: false,
  cancelMfaDialog: () => null,
};
