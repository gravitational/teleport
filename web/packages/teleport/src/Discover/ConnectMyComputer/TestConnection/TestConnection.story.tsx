/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
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

import { nodes } from 'teleport/Nodes/fixtures';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { UserContext } from 'teleport/User/UserContext';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';

import { TestConnection } from './TestConnection';

export default {
  title: 'Teleport/Discover/ConnectMyComputer/TestConnection',
  loaders: [mswLoader],
};

initialize();

const node = nodes[0];

const agentStepProps = {
  prevStep: () => {},
  nextStep: () => {},
  agentMeta: { resourceName: node.hostname, node, agentMatcherLabels: [] },
};

export const Story = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

Story.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.webRenewTokenPath, (req, res, ctx) =>
        res(ctx.json({}))
      ),
      rest.get(cfg.api.nodesPath, (req, res, ctx) =>
        res(ctx.json({ items: [node] }))
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
          ctx.json({ message: 'Could not renew session' })
        )
      ),
    ],
  },
};

const Provider = ({ children }) => {
  const ctx = createTeleportContext();

  const preferences = makeDefaultUserPreferences();
  const updatePreferences = () => Promise.resolve();
  const getClusterPinnedResources = () => Promise.resolve([]);
  const updateClusterPinnedResources = () => Promise.resolve();

  return (
    <MemoryRouter>
      <UserContext.Provider
        value={{
          preferences,
          updatePreferences,
          getClusterPinnedResources,
          updateClusterPinnedResources,
        }}
      >
        <ContextProvider ctx={ctx}>{children}</ContextProvider>
      </UserContext.Provider>
    </MemoryRouter>
  );
};
