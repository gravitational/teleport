/**
 * Copyright 2020 Gravitational, Inc.
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
import { AddNode } from './AddNode';

export default {
  title: 'Teleport/Nodes/Add',
};

export const Loaded = () => <AddNode {...props} />;

export const Processing = () => (
  <AddNode {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <AddNode
    {...props}
    attempt={{ status: 'failed', statusText: 'some error message' }}
  />
);

export const ManuallyProcessing = () => (
  <AddNode {...props} automatic={false} attempt={{ status: 'processing' }} />
);

export const ManuallyWithToken = () => <AddNode {...props} automatic={false} />;

export const ManuallyWithoutTokenLocal = () => (
  <AddNode {...props} automatic={false} attempt={{ status: 'failed' }} />
);

export const ManuallyWithoutTokenSSO = () => (
  <AddNode
    {...props}
    automatic={false}
    isAuthTypeLocal={false}
    attempt={{ status: 'failed' }}
  />
);

const props = {
  isAuthTypeLocal: true,
  onClose() {
    return null;
  },
  createJoinToken() {
    return Promise.resolve(null);
  },
  user: 'sam',
  automatic: true,
  setAutomatic: () => null,
  version: '5.0.0-dev',
  isEnterprise: true,
  script: 'some bash script',
  expiry: '4 hours',
  attempt: {
    status: 'success',
    statusText: '',
  } as any,
  token: 'some-join-token-hash',
};
