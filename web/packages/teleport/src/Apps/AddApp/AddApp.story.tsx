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
import { AddApp } from './AddApp';

export default {
  title: 'Teleport/Apps/Add',
};

export const Created = () => (
  <AddApp {...props} attempt={{ status: 'success' }} />
);

export const Loaded = () => {
  return <AddApp {...props} cmd="" />;
};

export const Processing = () => (
  <AddApp {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <AddApp
    {...props}
    attempt={{ status: 'failed', statusText: 'some error message' }}
  />
);

export const ManuallyLocal = () => <AddApp {...props} automatic={false} />;

export const ManuallySSO = () => (
  <AddApp {...props} automatic={false} isAuthTypeLocal={false} />
);

const props = {
  isEnterprise: false,
  isAuthTypeLocal: true,
  user: 'sam',
  automatic: true,
  setAutomatic: () => null,
  createToken: () => Promise.resolve(true),
  onClose: () => null,
  createJoinToken: () => Promise.resolve(null),
  version: '5.0.0-dev',
  cmd: `sudo bash -c "$(curl -fsSL 'http://localhost/scripts/86/install-app.sh?name=test&uri=http://myapp/')"`,
  expires: '1 hour',
  reset: () => null,
  attempt: {
    status: '',
    statusText: '',
  } as any,
};
