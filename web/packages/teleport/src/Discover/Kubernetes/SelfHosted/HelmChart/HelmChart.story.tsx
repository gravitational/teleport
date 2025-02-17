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
import { getUserContext } from 'teleport/mocks/contexts';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import { INTERNAL_RESOURCE_ID_LABEL_KEY } from 'teleport/services/joinToken';
import { DiscoverEventResource } from 'teleport/services/userEvent';

import HelmChart from './HelmChart';

const kubePathWithoutQuery = withoutQuery(cfg.api.kubernetesPath);

export default {
  title: 'Teleport/Discover/Kube/HelmChart',
  decorators: [
    Story => {
      // Reset request handlers added in individual stories.
      clearCachedJoinTokenResult([ResourceKind.Kubernetes]);
      return <Story />;
    },
  ],
};

export const Polling: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(kubePathWithoutQuery, async () => {
          await delay('infinite');
        }),
        http.post(cfg.api.discoveryJoinToken.createV2, () =>
          HttpResponse.json(rawJoinToken)
        ),
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
        http.get(kubePathWithoutQuery, () => {
          return HttpResponse.json({ items: [{}] });
        }),
        http.post(cfg.api.discoveryJoinToken.createV2, () =>
          HttpResponse.json(rawJoinToken)
        ),
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

// TODO(lisa): state will show up after 5 minutes, in order
// to reduce this time, requires rewriting component in a way
// that can mock the SHOW_HINT_TIMEOUT for window.setTimeout
export const PollingError: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(kubePathWithoutQuery, async () => {
          await delay('infinite');
        }),
        http.post(cfg.api.discoveryJoinToken.createV2, () =>
          HttpResponse.json(rawJoinToken)
        ),
      ],
    },
  },
  render() {
    return (
      <Provider interval={50}>
        <HelmChart />
      </Provider>
    );
  },
};

export const Processing: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.post(cfg.api.discoveryJoinToken.createV2, async () => {
          await delay('infinite');
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
        http.post(cfg.api.discoveryJoinToken.createV2, () =>
          HttpResponse.json(
            {
              error: { message: 'Whoops, something went wrong.' },
            },
            { status: 400 }
          )
        ),
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
      <ContextProvider ctx={createTeleportContext()}>
        <PingTeleportProvider
          interval={props.interval || 100000}
          resourceKind={ResourceKind.Kubernetes}
        >
          <DiscoverProvider mockCtx={discoverCtx}>
            {props.children}
          </DiscoverProvider>
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

const rawJoinToken = {
  id: 'some-id',
  roles: ['Node'],
  method: 'iam',
  suggestedLabels: [
    { name: INTERNAL_RESOURCE_ID_LABEL_KEY, value: 'some-value' },
  ],
};
