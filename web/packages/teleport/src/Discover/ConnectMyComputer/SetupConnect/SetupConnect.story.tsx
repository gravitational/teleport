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

import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';
import { withoutQuery } from 'web/packages/build/storybook';

import {
  OverrideUserAgent,
  UserAgent,
} from 'shared/components/OverrideUserAgent';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import { UserContext } from 'teleport/User/UserContext';

import { SetupConnect } from './SetupConnect';

const oneDay = 1000 * 60 * 60 * 24;

const setupConnectProps = {
  prevStep: () => {},
  nextStep: () => {},
  updateAgentMeta: () => {},
  // Set high default intervals and timeouts so that stories don't poll for no reason.
  pingInterval: oneDay,
  showHintTimeout: oneDay,
};

export default {
  title: 'Teleport/Discover/ConnectMyComputer/SetupConnect',
};

const noNodesHandler = http.get(withoutQuery(cfg.api.nodesPath), () =>
  HttpResponse.json({ items: [] })
);

export const macOS = () => {
  return (
    <OverrideUserAgent userAgent={UserAgent.macOS}>
      <Provider>
        <SetupConnect {...setupConnectProps} />
      </Provider>
    </OverrideUserAgent>
  );
};
macOS.parameters = {
  msw: {
    handlers: [noNodesHandler],
  },
};

export const Linux = () => {
  return (
    <OverrideUserAgent userAgent={UserAgent.Linux}>
      <Provider>
        <SetupConnect {...setupConnectProps} />
      </Provider>
    </OverrideUserAgent>
  );
};
Linux.parameters = {
  msw: {
    handlers: [noNodesHandler],
  },
};

export const Polling = () => {
  return (
    <Provider>
      <SetupConnect {...setupConnectProps} />
    </Provider>
  );
};
Polling.parameters = {
  msw: {
    handlers: [noNodesHandler],
  },
};

export const PollingSuccess = () => {
  return (
    <Provider>
      <SetupConnect {...setupConnectProps} pingInterval={5} />
    </Provider>
  );
};
PollingSuccess.parameters = {
  msw: {
    handlers: [
      http.get(
        withoutQuery(cfg.api.nodesPath),
        () => {
          return HttpResponse.json({ items: [] });
        },
        { once: true }
      ),
      http.get(withoutQuery(cfg.api.nodesPath), () => {
        return HttpResponse.json({ items: [{ id: '1234', hostname: 'foo' }] });
      }),
    ],
  },
};

export const HintTimeout = () => {
  return (
    <Provider>
      <SetupConnect {...setupConnectProps} showHintTimeout={1} />
    </Provider>
  );
};

HintTimeout.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.webRenewTokenPath, () => HttpResponse.json({})),
    ],
  },
};

const Provider = ({ children }) => {
  const ctx = createTeleportContext();
  // The proxy version is set mostly so that the download links point to actual artifacts.
  ctx.storeUser.state.cluster.proxyVersion = '14.1.0';

  const preferences = makeDefaultUserPreferences();
  const updatePreferences = () => Promise.resolve();
  const getClusterPinnedResources = () => Promise.resolve([]);
  const updateClusterPinnedResources = () => Promise.resolve();
  const updateDiscoverResourcePreferences = () => Promise.resolve();

  return (
    <MemoryRouter>
      <UserContext.Provider
        value={{
          preferences,
          updatePreferences,
          getClusterPinnedResources,
          updateClusterPinnedResources,
          updateDiscoverResourcePreferences,
        }}
      >
        <ContextProvider ctx={ctx}>{children}</ContextProvider>
      </UserContext.Provider>
    </MemoryRouter>
  );
};
