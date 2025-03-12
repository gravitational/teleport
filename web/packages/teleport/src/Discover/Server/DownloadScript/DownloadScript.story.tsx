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

import { StoryObj } from '@storybook/react';
import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';
import { withoutQuery } from 'web/packages/build/storybook';

import { ContextProvider, Context as TeleportContext } from 'teleport';
import cfg from 'teleport/config';
import { ResourceKind } from 'teleport/Discover/Shared';
import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { clearCachedJoinTokenResult } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { userContext } from 'teleport/Main/fixtures';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import {
  INTERNAL_RESOURCE_ID_LABEL_KEY,
  JoinToken,
} from 'teleport/services/joinToken';
import { DiscoverEventResource } from 'teleport/services/userEvent';
import { UserContextProvider } from 'teleport/User';

import DownloadScript from './DownloadScript';

const nodesPathWithoutQuery = withoutQuery(cfg.api.nodesPath);

export default {
  title: 'Teleport/Discover/Server/DownloadScripts',
  decorators: [
    Story => {
      clearCachedJoinTokenResult([ResourceKind.Server]);
      return <Story />;
    },
  ],
};

export const Polling: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(nodesPathWithoutQuery, () => {
          return delay('infinite');
        }),
        http.post(cfg.api.discoveryJoinToken.createV2, () => {
          return HttpResponse.json(joinToken);
        }),
      ],
    },
  },
  render() {
    return (
      <Provider>
        <DownloadScript prevStep={() => null} />
      </Provider>
    );
  },
};

export const PollingSuccess: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        // Use default fetch token handler defined in mocks/handlers
        http.get(nodesPathWithoutQuery, () => {
          return HttpResponse.json({ items: [{}] });
        }),
        http.post(cfg.api.discoveryJoinToken.createV2, () => {
          return HttpResponse.json(joinToken);
        }),
      ],
    },
  },
  render() {
    return (
      <Provider interval={5}>
        <DownloadScript prevStep={() => null} />
      </Provider>
    );
  },
};

// TODO(lisa): state will show up after 5 minutes, in order
// to reduce this time, requires rewriting component in a way
// that can mock the SHOW_HINT_TIMEOUT for window.setTimeout
export const PollingError: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(nodesPathWithoutQuery, () => {
          return delay('infinite');
        }),
        http.post(cfg.api.discoveryJoinToken.createV2, () => {
          return HttpResponse.json(joinToken);
        }),
      ],
    },
  },
  render() {
    return (
      <Provider interval={50}>
        <DownloadScript prevStep={() => null} />
      </Provider>
    );
  },
};

export const Processing: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.post(cfg.api.discoveryJoinToken.createV2, () => {
          return delay('infinite');
        }),
      ],
    },
  },
  render() {
    return (
      <Provider interval={5}>
        <DownloadScript prevStep={() => null} />
      </Provider>
    );
  },
};

export const Failed: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.post(cfg.api.discoveryJoinToken.createV2, () => {
          return HttpResponse.json(
            {
              error: { message: 'Whoops, something went wrong.' },
            },
            { status: 500 }
          );
        }),
      ],
    },
  },
  render() {
    return (
      <Provider>
        <DownloadScript prevStep={() => null} />
      </Provider>
    );
  },
};

const Provider = props => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {
      awsIntegration: {
        kind: IntegrationKind.AwsOidc,
        name: 'some-name',
        resourceType: 'integration',
        spec: {
          roleArn: 'arn:aws:iam::123456789012:role/test-role-arn',
          issuerS3Bucket: '',
          issuerS3Prefix: '',
        },
        statusCode: IntegrationStatusCode.Running,
      },
    },
    currentStep: 0,
    nextStep: () => null,
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: {
      name: 'kube',
      kind: ResourceKind.Kubernetes,
      icon: 'kube',
      keywords: [],
      event: DiscoverEventResource.Kubernetes,
    },
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
      <UserContextProvider>
        <ContextProvider ctx={ctx}>
          <PingTeleportProvider
            interval={props.interval || 100000}
            resourceKind={ResourceKind.Server}
          >
            <DiscoverProvider mockCtx={discoverCtx}>
              {props.children}
            </DiscoverProvider>
          </PingTeleportProvider>
        </ContextProvider>
      </UserContextProvider>
    </MemoryRouter>
  );
};

function createTeleportContext() {
  const ctx = new TeleportContext();

  ctx.isEnterprise = false;
  ctx.storeUser.setState(userContext);

  return ctx;
}

const joinToken: JoinToken = {
  id: 'some-id',
  roles: [],
  isStatic: true,
  expiry: new Date(),
  method: 'local',
  safeName: '',
  content: '',
  suggestedLabels: [
    { name: INTERNAL_RESOURCE_ID_LABEL_KEY, value: 'some-internal' },
  ],
};
