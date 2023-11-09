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

import {
  OverrideUserAgent,
  UserAgent,
} from 'shared/components/OverrideUserAgent';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { UserContext } from 'teleport/User/UserContext';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';

import { SetupConnect } from './SetupConnect';

const { worker, rest } = window.msw;

const oneDay = 1000 * 60 * 60 * 24;

const setupConnectProps = {
  prevStep: () => {},
  nextStep: () => {},
  // Set high default intervals and timeouts so that stories don't poll for no reason.
  pingInterval: oneDay,
  showHintTimeout: oneDay,
};

export default {
  title: 'Teleport/Discover/ConnectMyComputer/SetupConnect',
  decorators: [
    Story => {
      worker.resetHandlers();
      return <Story />;
    },
  ],
};

const workerNoNodes = () => {
  worker.use(
    rest.get(cfg.api.nodesPath, (req, res, ctx) => res(ctx.json({ items: [] })))
  );
};

export const macOS = () => {
  workerNoNodes();
  return (
    <OverrideUserAgent userAgent={UserAgent.macOS}>
      <Provider>
        <SetupConnect {...setupConnectProps} />
      </Provider>
    </OverrideUserAgent>
  );
};

export const Linux = () => {
  workerNoNodes();
  return (
    <OverrideUserAgent userAgent={UserAgent.Linux}>
      <Provider>
        <SetupConnect {...setupConnectProps} />
      </Provider>
    </OverrideUserAgent>
  );
};

export const Polling = () => {
  workerNoNodes();

  return (
    <Provider>
      <SetupConnect {...setupConnectProps} />
    </Provider>
  );
};

export const PollingSuccess = () => {
  worker.use(
    rest.get(cfg.api.nodesPath, (req, res, ctx) => {
      return res(ctx.json({ items: [{ id: '1234', hostname: 'foo' }] }));
    })
  );
  worker.use(
    rest.get(cfg.api.nodesPath, (req, res, ctx) => {
      return res.once(ctx.json({ items: [] }));
    })
  );

  return (
    <Provider>
      <SetupConnect {...setupConnectProps} pingInterval={5} />
    </Provider>
  );
};

export const HintTimeout = () => {
  workerNoNodes();

  return (
    <Provider>
      <SetupConnect {...setupConnectProps} showHintTimeout={1} />
    </Provider>
  );
};

const Provider = ({ children }) => {
  const ctx = createTeleportContext();
  // The proxy version is set mostly so that the download links point to actual artifacts.
  ctx.storeUser.state.cluster.proxyVersion = '14.1.0';

  const preferences = makeDefaultUserPreferences();
  const updatePreferences = () => Promise.resolve();
  const getClusterPinnedResources = () => Promise.resolve([]);
  const updateClusterPinnedResources = () => Promise.resolve();

  return (
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
  );
};
