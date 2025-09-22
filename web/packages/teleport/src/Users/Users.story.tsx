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

import type { StoryObj } from '@storybook/react-vite';
import { delay } from 'msw';

import { TeleportProviderBasic } from 'teleport/mocks/providers';
import {
  errorGetUsers,
  handleGetUsers,
  successGetUsers,
} from 'teleport/test/helpers/users';

import { Users } from './Users';

export default {
  title: 'Teleport/Users',
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

export const Loaded: StoryObj = {
  parameters: {
    msw: {
      handlers: [successGetUsers(users)],
    },
  },
  render() {
    return (
      <TeleportProviderBasic>
        <Users {...sample} />
      </TeleportProviderBasic>
    );
  },
};

export const UsersNotEqualMauNotice: StoryObj = {
  parameters: {
    msw: {
      handlers: [successGetUsers(users)],
    },
  },
  render() {
    return (
      <TeleportProviderBasic>
        <Users {...sample} showMauInfo={true} />
      </TeleportProviderBasic>
    );
  },
};

export const Processing: StoryObj = {
  parameters: {
    msw: {
      handlers: [handleGetUsers(async () => await delay('infinite'))],
    },
  },
  render() {
    return (
      <TeleportProviderBasic>
        <Users {...sample} />
      </TeleportProviderBasic>
    );
  },
};

export const Failed: StoryObj = {
  parameters: {
    msw: {
      handlers: [errorGetUsers('Something went wrong')],
    },
  },
  render() {
    return (
      <TeleportProviderBasic>
        <Users {...sample} />
      </TeleportProviderBasic>
    );
  },
};

const sample = {
  attempt: {
    isProcessing: false,
    isFailed: false,
    isSuccess: true,
    message: '',
  },
  users: users,
  fetch: async () =>
    Promise.resolve({
      items: users,
      startKey: '',
    }),
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
