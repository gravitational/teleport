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

import { rest } from 'msw';

import { Context as TeleportContext, ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { ResourceKind } from 'teleport/Discover/Shared';
import { clearCachedJoinTokenResult } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { getUserContext } from 'teleport/mocks/contexts';

import HelmChart from './HelmChart';

export default {
  title: 'Teleport/Discover/Kube/HelmChart',
  decorators: [
    Story => {
      clearCachedJoinTokenResult([ResourceKind.Kubernetes]);
      return <Story />;
    },
  ],
};

export const Init = () => {
  return (
    <Provider>
      <HelmChart />
    </Provider>
  );
};

export const Polling: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        rest.get(cfg.api.kubernetesPath, (req, res, ctx) => {
          return res(ctx.delay('infinite'));
        }),
      ],
    },
  },
  render() {
    return (
      <Provider>
        <HelmChart />
      </Provider>
    );
  },
};

export const PollingSuccess: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        rest.get(cfg.api.kubernetesPath, (req, res, ctx) => {
          return res(ctx.json({ items: [{}] }));
        }),
      ],
    },
  },
  render() {
    return (
      <Provider interval={5}>
        <HelmChart />
      </Provider>
    );
  },
};

export const LoadedPollingErrorWithIgs: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        rest.get(cfg.api.kubernetesPath, (req, res, ctx) => {
          return res(ctx.delay('infinite'));
        }),
      ],
    },
  },
  render() {
    return (
      <Provider timeout={50}>
        <HelmChart />
      </Provider>
    );
  },
};

export const Processing: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        rest.post(cfg.api.joinTokenPath, (req, res, ctx) => {
          return res(ctx.delay('infinite'));
        }),
      ],
    },
  },
  render() {
    return (
      <Provider interval={5}>
        <HelmChart />
      </Provider>
    );
  },
};

export const Failed: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        rest.post(cfg.api.joinTokenPath, (req, res, ctx) => {
          return res.once(ctx.status(500));
        }),
      ],
    },
  },
  render() {
    return (
      <Provider>
        <HelmChart />
      </Provider>
    );
  },
};

const Provider = props => {
  const ctx = createTeleportContext();

  return (
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'database' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <PingTeleportProvider
          interval={props.interval || 100000}
          resourceKind={ResourceKind.Kubernetes}
        >
          {props.children}
        </PingTeleportProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

function createTeleportContext() {
  const ctx = new TeleportContext();

  ctx.isEnterprise = false;
  ctx.storeUser.setState(getUserContext());

  return ctx;
}
