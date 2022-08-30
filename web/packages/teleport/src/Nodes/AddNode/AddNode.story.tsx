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
import { Attempt } from 'shared/hooks/useAttemptNext';

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
  <AddNode {...props} method="manual" attempt={{ status: 'processing' }} />
);

export const ManuallyWithToken = () => <AddNode {...props} method="manual" />;

export const ManuallyWithoutTokenLocal = () => (
  <AddNode {...props} method="manual" attempt={{ status: 'failed' }} />
);

export const ManuallyWithoutTokenSSO = () => (
  <AddNode
    {...props}
    method="manual"
    isAuthTypeLocal={false}
    attempt={{ status: 'failed' }}
  />
);

export const IamWithoutToken = () => (
  <AddNode {...props} method="iam" iamJoinToken={null} />
);

export const IamWithToken = () => <AddNode {...props} method="iam" />;

export const IamProcessing = () => (
  <AddNode
    {...props}
    method="iam"
    iamJoinToken={null}
    iamAttempt={{ status: 'processing' }}
  />
);

export const IamFailed = () => (
  <AddNode
    {...props}
    method="iam"
    iamJoinToken={null}
    iamAttempt={{ status: 'failed', statusText: 'some err' }}
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
  method: 'automatic' as any,
  setMethod: () => null,
  setAutomatic: () => null,
  version: '5.0.0-dev',
  isEnterprise: true,
  attempt: {
    status: 'success',
    statusText: '',
  } as Attempt,
  iamAttempt: {
    status: 'success',
    statusText: '',
  } as Attempt,
  token: {
    id: 'some-join-token-hash',
    expiryText: '4 hours',
    expiry: new Date(),
  },
  iamJoinToken: {
    id: 'some-join-token-hash',
    expiryText: '1000 years',
    expiry: new Date(),
  },
  createIamJoinToken() {
    return Promise.resolve(null);
  },
};
