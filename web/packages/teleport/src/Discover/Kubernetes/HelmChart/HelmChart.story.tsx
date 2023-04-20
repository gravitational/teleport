/**
 * Copyright 2022 Gravitational, Inc.
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
import { MemoryRouter } from 'react-router';

import { Context as TeleportContext, ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { ResourceKind } from 'teleport/Discover/Shared';
import { clearCachedJoinTokenResult } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { getUserContext } from 'teleport/mocks/contexts';

import HelmChart from './HelmChart';

const { worker, rest } = window.msw;

export default {
  title: 'Teleport/Discover/Kube/HelmChart',
  decorators: [
    Story => {
      // Reset request handlers added in individual stories.
      worker.resetHandlers();
      clearCachedJoinTokenResult(ResourceKind.Kubernetes);
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

export const Polling = () => {
  // Use default fetch token handler defined in mocks/handlers

  worker.use(
    rest.get(cfg.api.kubernetesPath, (req, res, ctx) => {
      return res(ctx.delay('infinite'));
    })
  );
  return (
    <Provider>
      <HelmChart />
    </Provider>
  );
};

export const PollingSuccess = () => {
  // Use default fetch token handler defined in mocks/handlers

  worker.use(
    rest.get(cfg.api.kubernetesPath, (req, res, ctx) => {
      return res(ctx.json({ items: [{}] }));
    })
  );
  return (
    <Provider interval={5}>
      <HelmChart />
    </Provider>
  );
};

export const PollingError = () => {
  // Use default fetch token handler defined in mocks/handlers

  worker.use(
    rest.get(cfg.api.kubernetesPath, (req, res, ctx) => {
      return res(ctx.delay('infinite'));
    })
  );
  return (
    <Provider timeout={50}>
      <HelmChart />
    </Provider>
  );
};

export const Processing = () => {
  worker.use(
    rest.post(cfg.api.joinTokenPath, (req, res, ctx) => {
      return res(ctx.delay('infinite'));
    })
  );
  return (
    <Provider interval={5}>
      <HelmChart />
    </Provider>
  );
};

export const Failed = () => {
  worker.use(
    rest.post(cfg.api.joinTokenPath, (req, res, ctx) => {
      return res.once(ctx.status(500));
    })
  );
  return (
    <Provider>
      <HelmChart />
    </Provider>
  );
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
