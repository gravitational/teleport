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
import { initialize, mswLoader } from 'msw-storybook-addon';
import { rest } from 'msw';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { nodes } from 'teleport/Nodes/fixtures';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  DiscoverProvider,
  DiscoverContextState,
} from 'teleport/Discover/useDiscover';

import { TestConnection } from './TestConnection';

export default {
  title: 'Teleport/Discover/ConnectMyComputer/TestConnection',
  loaders: [mswLoader],
};

initialize();

const node = { ...nodes[0] };
node.sshLogins = [
  ...node.sshLogins,
  'george_washington_really_long_name_testing',
];
const agentStepProps = {
  prevStep: () => {},
  nextStep: () => {},
  agentMeta: { resourceName: node.hostname, node, agentMatcherLabels: [] },
};

export const SingleLogin = () => (
  <Provider>
    <TestConnection {...agentStepProps} />
  </Provider>
);

SingleLogin.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.webRenewTokenPath, (req, res, ctx) =>
        res(ctx.json({}))
      ),
      rest.get(cfg.api.connectMyComputerLoginsPath, (req, res, ctx) =>
        res(ctx.json({ logins: ['foo'] }))
      ),
    ],
  },
};

export const MultipleLogins = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

MultipleLogins.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.webRenewTokenPath, (req, res, ctx) =>
        res(ctx.json({}))
      ),
      rest.get(cfg.api.connectMyComputerLoginsPath, (req, res, ctx) =>
        res(
          ctx.json({
            logins: [
              'foo',
              'bar',
              'baz',
              'czesława_maria_de_domo_cieślak_primo_voto_gospodarek_secundo_voto_kowalczyk',
            ],
          })
        )
      ),
    ],
  },
};

export const NoLogins = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

NoLogins.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.webRenewTokenPath, (req, res, ctx) =>
        res(ctx.json({}))
      ),
      rest.get(cfg.api.connectMyComputerLoginsPath, (req, res, ctx) =>
        res(ctx.json({ logins: [] }))
      ),
    ],
  },
};

export const NoRole = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

NoRole.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.webRenewTokenPath, (req, res, ctx) =>
        res(ctx.json({}))
      ),
      rest.get(cfg.api.connectMyComputerLoginsPath, (req, res, ctx) =>
        res(ctx.status(404), ctx.json({ error: { message: 'No role found' } }))
      ),
    ],
  },
};

export const ReloadUserProcessing = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

ReloadUserProcessing.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.webRenewTokenPath, (req, res, ctx) =>
        res(ctx.delay('infinite'))
      ),
    ],
  },
};

export const ReloadUserError = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

ReloadUserError.parameters = {
  msw: {
    handlers: [
      // The first handler returns an error immediately. Subsequent requests return after a delay so
      // that we can show a spinner after clicking on "Retry".
      rest.post(cfg.api.webRenewTokenPath, (req, res, ctx) =>
        res.once(
          ctx.status(500),
          ctx.json({ message: 'Could not renew session' })
        )
      ),
      rest.post(cfg.api.webRenewTokenPath, (req, res, ctx) =>
        res(
          ctx.delay(1000),
          ctx.status(500),
          ctx.json({ error: { message: 'Could not renew session' } })
        )
      ),
    ],
  },
};

const Provider = ({ children }) => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    ...agentStepProps,
    currentStep: 0,
    onSelectResource: () => null,
    resourceSpec: undefined,
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: () => null,
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
  };

  return (
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'server' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <DiscoverProvider mockCtx={discoverCtx}>{children}</DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
