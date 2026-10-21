/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import type { Meta } from '@storybook/react-vite';
import { delay, http, HttpResponse } from 'msw';
import { useEffect, type ComponentProps } from 'react';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';

import { LoginScopePicker as Component } from './LoginScopePicker';

type StoryProps = ComponentProps<typeof Component> & {
  username: string;
  assignedScopes: string[];
};

const meta: Meta<StoryProps> = {
  title: 'Teleport/Scopes',
  args: {
    username: 'alice',
    assignedScopes: ['/prod', '/staging/west', '/staging/west/team-a'],
  },
};

export default meta;

export function LoginScopePicker({ username }: StoryProps) {
  const ctx = createTeleportContext();

  useEffect(() => {
    ctx.storeUser.setState({
      ...ctx.storeUser.state,
      username,
    });
  }, [ctx, username]);

  return (
    <TeleportProviderBasic teleportCtx={ctx}>
      <Component />
    </TeleportProviderBasic>
  );
}

LoginScopePicker.beforeEach = ({ args, msw }) => {
  msw.use(
    http.get(cfg.api.listAssignedScopes, () =>
      HttpResponse.json({ scopes: args.assignedScopes })
    )
  );
};

export function LoginScopePickerPending({ username }: StoryProps) {
  const ctx = createTeleportContext();

  useEffect(() => {
    ctx.storeUser.setState({
      ...ctx.storeUser.state,
      username,
    });
  }, [ctx, username]);

  return (
    <TeleportProviderBasic teleportCtx={ctx}>
      <Component />
    </TeleportProviderBasic>
  );
}

LoginScopePickerPending.beforeEach = ({ msw }) => {
  msw.use(http.get(cfg.api.listAssignedScopes, () => delay('infinite')));
};

export function LoginScopePickerError({ username }: StoryProps) {
  const ctx = createTeleportContext();

  useEffect(() => {
    ctx.storeUser.setState({
      ...ctx.storeUser.state,
      username,
    });
  }, [ctx, username]);

  return (
    <TeleportProviderBasic teleportCtx={ctx}>
      <Component />
    </TeleportProviderBasic>
  );
}

LoginScopePickerError.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.api.listAssignedScopes, () =>
      HttpResponse.json({ error: { message: 'bad request' } }, { status: 400 })
    )
  );
};
