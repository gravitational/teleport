/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { createTeleportContext, getAcl } from 'teleport/mocks/contexts';

import { AwsAccount } from './AwsAccount';

export default {
  title: 'Teleport/Discover/Shared/AwsAccount',
};

const handlers = [
  http.get(cfg.getIntegrationsUrl(), () =>
    HttpResponse.json({
      items: [
        {
          name: 'aws-oidc-1',
          subKind: 'aws-oidc',
          awsoidc: {
            roleArn: 'arn:aws:iam::123456789012:role/test1',
          },
        },
      ],
    })
  ),
  http.get(cfg.api.unifiedResourcesPath, () =>
    HttpResponse.json({ agents: [{ name: 'app1' }] })
  ),
];

export const Success = () => <Component />;
Success.parameters = {
  msw: {
    handlers,
  },
};

export const Loading = () => <Component />;
Loading.parameters = {
  msw: {
    handlers: [http.get(cfg.getIntegrationsUrl(), () => delay('infinite'))],
  },
};

export const Failed = () => <Component />;
Failed.parameters = {
  msw: {
    handlers: [
      http.post(cfg.getIntegrationsUrl(), () =>
        HttpResponse.json(
          {
            message: 'some kind of error',
          },
          { status: 403 }
        )
      ),
    ],
  },
};

export const NoPerm = () => <Component noAccess={true} />;

const Component = ({ noAccess = false }: { noAccess?: boolean }) => {
  const ctx = createTeleportContext();
  ctx.storeUser.state.acl = getAcl({ noAccess });
  const discoverCtx: DiscoverContextState = {
    agentMeta: {},
    currentStep: 0,
    nextStep: () => null,
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: {} as any,
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: () => null,
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
  };

  cfg.proxyCluster = 'localhost';
  return (
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'server' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <DiscoverProvider mockCtx={discoverCtx}>
          <AwsAccount />
        </DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
