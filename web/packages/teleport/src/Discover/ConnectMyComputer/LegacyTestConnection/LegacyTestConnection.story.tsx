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
import { initialize, mswLoader } from 'msw-storybook-addon';
import { rest } from 'msw';

import { nodes } from 'teleport/Nodes/fixtures';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { UserContext } from 'teleport/User/UserContext';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';

import { LegacyTestConnection } from './LegacyTestConnection';

export default {
  title: 'Teleport/Discover/ConnectMyComputer/LegacyTestConnection',
  loaders: [mswLoader],
};

initialize();

const node = nodes[0];

const agentStepProps = {
  prevStep: () => {},
  nextStep: () => {},
  agentMeta: { resourceName: node.hostname, node, agentMatcherLabels: [] },
};

export const SingleLogin = () => {
  return (
    <Provider>
      <LegacyTestConnection {...agentStepProps} />
    </Provider>
  );
};

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
      <LegacyTestConnection {...agentStepProps} />
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
        res(ctx.json({ logins: ['foo', 'bar', 'baz'] }))
      ),
    ],
  },
};

export const NoLogins = () => {
  return (
    <Provider>
      <LegacyTestConnection {...agentStepProps} />
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
      <LegacyTestConnection {...agentStepProps} />
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
        // TODO Check how our error responses look like.
        res(ctx.status(404), ctx.text('Whoops no role found'))
      ),
    ],
  },
};

export const ReloadUserProcessing = () => {
  return (
    <Provider>
      <LegacyTestConnection {...agentStepProps} />
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
      <LegacyTestConnection {...agentStepProps} />
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
