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

import { Users } from './Users';

export default {
  title: 'Teleport/Users',
};

export const Processing = () => {
  const attempt = {
    isProcessing: true,
    isFailed: false,
    isSuccess: false,
    message: '',
  };
  return <Users {...sample} attempt={attempt} />;
};

export const Loaded = () => {
  return <Users {...sample} />;
};

export const Failed = () => {
  const attempt = {
    isProcessing: false,
    isFailed: true,
    isSuccess: false,
    message: 'some error message',
  };
  return <Users {...sample} attempt={attempt} />;
};

const users = [
  {
    name: 'cikar@egaposci.me',
    roles: ['admin'],
    authType: 'teleport local user',
    isLocal: true,
  },
  {
    name: 'hi@nen.pa',
    roles: ['ruhh', 'admin'],
    authType: 'teleport local user',
    isLocal: true,
  },
  {
    name: 'ziat@uthatebo.sl',
    roles: ['kaco', 'ziuzzow', 'admin'],
    authType: 'github',
    isLocal: false,
  },
  {
    name: 'pamkad@ukgir.ki',
    roles: ['vuit', 'vedkonm', 'valvapel'],
    authType: 'saml',
    isLocal: false,
  },
  {
    name: 'jap@kosusfaw.mp',
    roles: ['ubip', 'duzjadj', 'dupiwuzocafe', 'abc', 'anavebikilonim'],
    authType: 'oidc',
    isLocal: false,
  },
  {
    name: 'azesotil@jevig.org',
    roles: ['tugu'],
    authType: 'teleport local user',
    isLocal: true,
  },
];

const roles = ['admin', 'testrole'];

const sample = {
  attempt: {
    isProcessing: false,
    isFailed: false,
    isSuccess: true,
    message: '',
  },
  users: users,
  roles: roles,
  operation: {
    type: 'none',
    user: null,
  } as any,
  onStartCreate: () => null,
  onStartDelete: () => null,
  onStartEdit: () => null,
  onStartReset: () => null,
  onClose: () => null,
  onCreate: () => null,
  onDelete: () => null,
  onUpdate: () => null,
  onReset: () => null,
};
