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

import React from 'react';
import { MemoryRouter } from 'react-router';

import { StoryObj } from '@storybook/react';

import { delay } from 'msw';

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';

import {
  errorGetUsers,
  handleGetUsers,
  successGetUsers,
} from 'teleport/test/handlers/users';
import cfg from 'teleport/config';

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

export const Processing: StoryObj = {
  parameters: {
    msw: {
      handlers: [handleGetUsers(async () => await delay('infinite'))],
    },
  },
  render() {
    const ctx = createTeleportContext();

    return (
      <ContextProvider ctx={ctx}>
        <MemoryRouter>
          <Users />
        </MemoryRouter>
      </ContextProvider>
    );
  },
};

export const Loaded: StoryObj = {
  parameters: {
    msw: {
      handlers: [successGetUsers(users)],
    },
  },
  render() {
    const ctx = createTeleportContext();

    return (
      <ContextProvider ctx={ctx}>
        <MemoryRouter>
          <Users />
        </MemoryRouter>
      </ContextProvider>
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
    cfg.isUsageBasedBilling = true;

    const ctx = createTeleportContext();

    return (
      <ContextProvider ctx={ctx}>
        <MemoryRouter>
          <Users />
        </MemoryRouter>
      </ContextProvider>
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
    const ctx = createTeleportContext();

    return (
      <ContextProvider ctx={ctx}>
        <MemoryRouter>
          <Users />
        </MemoryRouter>
      </ContextProvider>
    );
  },
};
