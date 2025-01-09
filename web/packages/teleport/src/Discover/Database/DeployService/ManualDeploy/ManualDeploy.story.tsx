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

import { ContextProvider, Context as TeleportContext } from 'teleport';
import cfg from 'teleport/config';
import {
  DatabaseEngine,
  DatabaseLocation,
} from 'teleport/Discover/SelectResource';
import { ResourceKind } from 'teleport/Discover/Shared';
import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { getUserContext } from 'teleport/mocks/contexts';
import { INTERNAL_RESOURCE_ID_LABEL_KEY } from 'teleport/services/joinToken';

import ManualDeploy from './ManualDeploy';

const DEFAULT_PING_INTERVAL = 1000 * 100; // 100 seconds

export default {
  title: 'Teleport/Discover/Database/Deploy/Manual',
};

export const Init = () => {
  return (
    <Provider>
      <ManualDeploy />
    </Provider>
  );
};
Init.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.discoveryJoinToken.createV2, () =>
        HttpResponse.json(rawJoinToken)
      ),
    ],
  },
};

export const InitWithLabels = () => {
  return (
    <Provider
      agentMeta={{
        agentMatcherLabels: [
          { name: 'env', value: 'staging' },
          { name: 'os', value: 'windows' },
        ],
      }}
    >
      <ManualDeploy />
    </Provider>
  );
};
InitWithLabels.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.discoveryJoinToken.createV2, () =>
        HttpResponse.json({})
      ),
    ],
  },
};

const Provider = props => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {
      resourceName: 'db-name',
      agentMatcherLabels: [],
      db: {} as any,
      selectedAwsRdsDb: {} as any,
      ...props.agentMeta,
    },
    currentStep: 0,
    nextStep: () => null,
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: {
      dbMeta: {
        location: DatabaseLocation.Aws,
        engine: DatabaseEngine.AuroraMysql,
      },
    } as any,
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
        { pathname: cfg.routes.discover, state: { entity: 'database' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <FeaturesContextProvider value={[]}>
          <DiscoverProvider mockCtx={discoverCtx}>
            <PingTeleportProvider
              interval={props.interval || DEFAULT_PING_INTERVAL}
              resourceKind={ResourceKind.Database}
            >
              {props.children}
            </PingTeleportProvider>
          </DiscoverProvider>
        </FeaturesContextProvider>
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

const rawJoinToken = {
  id: 'some-id',
  roles: ['Node'],
  method: 'iam',
  suggestedLabels: [
    { name: INTERNAL_RESOURCE_ID_LABEL_KEY, value: 'some-value' },
  ],
};
