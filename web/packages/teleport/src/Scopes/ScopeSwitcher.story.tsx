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

import { Flex } from '@gravitational/design-system';
import type { Meta } from '@storybook/react-vite';
import { delay, http, HttpResponse } from 'msw';
import { useEffect, type ComponentProps } from 'react';
import styled from 'styled-components';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';

import { ScopeSwitcher as Component } from './ScopeSwitcher';

type StoryProps = ComponentProps<typeof Component> & {
  username: string;
  assignedScopes: string[];
  currentScope: string;
};

const meta: Meta<StoryProps> = {
  title: 'Teleport/Scopes',
  args: {
    username: 'alice',
    assignedScopes: ['/prod', '/staging/west', '/staging/west/team-a'],
    currentScope: '',
  },
};

export default meta;

export function ScopeSwitcher({ username, currentScope }: StoryProps) {
  const ctx = createTeleportContext();

  useEffect(() => {
    ctx.storeUser.setState({
      ...ctx.storeUser.state,
      username,
    });
  }, [ctx, username]);

  return (
    <TeleportProviderBasic teleportCtx={ctx}>
      <Container>
        <Component scope={currentScope} />
      </Container>
    </TeleportProviderBasic>
  );
}

ScopeSwitcher.beforeEach = ({ args, msw }) => {
  msw.use(
    http.get(cfg.api.listAssignedScopes, () =>
      HttpResponse.json({ scopes: args.assignedScopes })
    )
  );
};

export function ScopeSwitcherPending({ username, currentScope }: StoryProps) {
  const ctx = createTeleportContext();

  useEffect(() => {
    ctx.storeUser.setState({
      ...ctx.storeUser.state,
      username,
    });
  }, [ctx, username]);

  return (
    <TeleportProviderBasic teleportCtx={ctx}>
      <Container>
        <Component scope={currentScope} />
      </Container>
    </TeleportProviderBasic>
  );
}

ScopeSwitcherPending.beforeEach = ({ msw }) => {
  msw.use(http.get(cfg.api.listAssignedScopes, () => delay('infinite')));
};

export function ScopeSwitcherError({ username, currentScope }: StoryProps) {
  const ctx = createTeleportContext();

  useEffect(() => {
    ctx.storeUser.setState({
      ...ctx.storeUser.state,
      username,
    });
  }, [ctx, username]);

  return (
    <TeleportProviderBasic teleportCtx={ctx}>
      <Container>
        <Component scope={currentScope} />
      </Container>
    </TeleportProviderBasic>
  );
}

ScopeSwitcherError.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.api.listAssignedScopes, () =>
      HttpResponse.json({ error: { message: 'bad request' } }, { status: 400 })
    )
  );
};

const Container = styled(Flex)`
  height: ${p => p.theme.topBarHeight[0]}px;
`;
