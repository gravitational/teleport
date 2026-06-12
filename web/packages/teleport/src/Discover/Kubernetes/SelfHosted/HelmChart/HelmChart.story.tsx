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

import { StoryObj } from '@storybook/react-vite';
import { delay, http, HttpResponse } from 'msw';
import { PropsWithChildren } from 'react';
import { withoutQuery } from 'web/packages/build/storybook';

import cfg from 'teleport/config';
import {
  RequiredDiscoverProviders,
  resourceSpecSelfHostedKube,
} from 'teleport/Discover/Fixtures/fixtures';
import { ResourceKind } from 'teleport/Discover/Shared';
import { clearCachedJoinTokenResult } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { AgentMeta } from 'teleport/Discover/useDiscover';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import { INTERNAL_RESOURCE_ID_LABEL_KEY } from 'teleport/services/joinToken';

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

const agentMeta: AgentMeta = {
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
};

const Provider: React.FC<PropsWithChildren<{ interval?: number }>> = props => {
  return (
    <RequiredDiscoverProviders
      interval={props.interval}
      agentMeta={agentMeta}
      resourceSpec={resourceSpecSelfHostedKube}
    >
      {props.children}
    </RequiredDiscoverProviders>
  );
};

const rawJoinToken = {
  id: 'some-id',
  roles: ['Node'],
  method: 'iam',
  suggestedLabels: [
    { name: INTERNAL_RESOURCE_ID_LABEL_KEY, value: 'some-value' },
  ],
};
