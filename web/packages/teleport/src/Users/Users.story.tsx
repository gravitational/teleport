/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { MemoryRouter } from 'react-router';

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
  return (
    <MemoryRouter>
      <Users {...sample} attempt={attempt} />
    </MemoryRouter>
  );
};

export const Loaded = () => {
  return (
    <MemoryRouter>
      <Users {...sample} />
    </MemoryRouter>
  );
};

export const UsersNotEqualMauNotice = () => {
  return (
    <MemoryRouter>
      <Users {...sample} showMauInfo={true} />
    </MemoryRouter>
  );
};

export const Failed = () => {
  const attempt = {
    isProcessing: false,
    isFailed: true,
    isSuccess: false,
    message: 'some error message',
  };
  return (
    <MemoryRouter>
      <Users {...sample} attempt={attempt} />
    </MemoryRouter>
  );
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
  {
    name: 'bot-little-robot',
    roles: ['bot-little-robot'],
    authType: 'teleport local user',
    isLocal: true,
    isBot: true,
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
  fetchRoles: async (input: string) => roles.filter(r => r.includes(input)),
  operation: {
    type: 'none',
    user: null,
  } as any,
  inviteCollaboratorsOpen: false,
  emailPasswordResetOpen: false,
  onStartCreate: () => null,
  onStartDelete: () => null,
  onStartEdit: () => null,
  onStartReset: () => null,
  onStartInviteCollaborators: () => null,
  onStartEmailResetPassword: () => null,
  onClose: () => null,
  onCreate: () => null,
  onDelete: () => null,
  onUpdate: () => null,
  onReset: () => null,
  onInviteCollaboratorsClose: () => null,
  InviteCollaborators: null,
  onEmailPasswordResetClose: () => null,
  EmailPasswordReset: null,
  showMauInfo: false,
  onDismissUsersMauNotice: () => null,
  canEditUsers: true,
  usersAcl: {
    read: true,
    edit: false,
    remove: true,
    list: true,
    create: true,
  },
};
