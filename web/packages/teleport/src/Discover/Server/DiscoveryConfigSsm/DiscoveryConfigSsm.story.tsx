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
import { useEffect } from 'react';

import { Info } from 'design/Alert';

import cfg from 'teleport/config';
import {
  RequiredDiscoverProviders,
  resourceSpecAwsEc2Ssm,
} from 'teleport/Discover/Fixtures/fixtures';
import { AgentMeta, AutoDiscovery } from 'teleport/Discover/useDiscover';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { DiscoveryConfigSsm } from './DiscoveryConfigSsm';

const defaultIsCloud = cfg.isCloud;
export default {
  title: 'Teleport/Discover/Server/EC2/DiscoveryConfigSsm',
  decorators: [
    Story => {
      useEffect(() => {
        // Clean up
        return () => {
          cfg.isCloud = defaultIsCloud;
        };
      }, []);
      return <Story />;
    },
  ],
};

export const SuccessCloud = () => {
  cfg.isCloud = true;
  return <Component />;
};
SuccessCloud.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.discoveryJoinToken.createV2, () =>
        HttpResponse.json({ id: 'token-id' })
      ),
      http.post(cfg.api.discoveryConfigPath, () =>
        HttpResponse.json({ name: 'discovery-cfg-name' })
      ),
    ],
  },
};

export const SuccessSelfHosted = () => (
  <Component
    autoDiscovery={{
      config: {
        name: 'some-name',
        aws: [],
        discoveryGroup: 'some-group',
      },
    }}
  />
);
SuccessSelfHosted.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.discoveryJoinToken.createV2, () =>
        HttpResponse.json({ id: 'token-id' })
      ),
      http.post(cfg.api.discoveryConfigPath, () =>
        HttpResponse.json({ name: 'discovery-cfg-name' })
      ),
    ],
  },
};

export const Loading = () => {
  cfg.isCloud = true;
  return <Component />;
};
Loading.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.discoveryJoinToken.createV2, () =>
        HttpResponse.json({ id: 'token-id' })
      ),
      http.post(cfg.api.discoveryConfigPath, () => delay('infinite')),
    ],
  },
};

export const Failed = () => {
  cfg.isCloud = true;
  return <Component />;
};
Failed.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.discoveryJoinToken.createV2, () =>
        HttpResponse.json({ id: 'token-id' })
      ),
      http.post(cfg.api.discoveryConfigPath, () =>
        HttpResponse.json(
          {
            message: 'Some kind of error message',
          },
          { status: 403 }
        )
      ),
    ],
  },
};

const Component = ({
  autoDiscovery = undefined,
}: {
  autoDiscovery?: AutoDiscovery;
}) => {
  const agentMeta: AgentMeta = {
    resourceName: 'aws-console',
    agentMatcherLabels: [],
    awsIntegration: {
      kind: IntegrationKind.AwsOidc,
      name: 'some-oidc-name',
      resourceType: 'integration',
      spec: {
        roleArn: 'arn:aws:iam::123456789012:role/test-role-arn',
        issuerS3Bucket: '',
        issuerS3Prefix: '',
      },
      statusCode: IntegrationStatusCode.Running,
    },
    autoDiscovery,
  };

  return (
    <RequiredDiscoverProviders
      agentMeta={agentMeta}
      resourceSpec={resourceSpecAwsEc2Ssm}
    >
      <Info>Devs: Click next to see next state</Info>
      <DiscoveryConfigSsm />
    </RequiredDiscoverProviders>
  );
};
