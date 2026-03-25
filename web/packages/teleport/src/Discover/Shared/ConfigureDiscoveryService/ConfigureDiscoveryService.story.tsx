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

import { http, HttpResponse } from 'msw';

import { Info } from 'design/Alert';

import cfg from 'teleport/config';
import { resourceSpecAwsRdsPostgres } from 'teleport/Discover/Fixtures/databases';
import {
  RequiredDiscoverProviders,
  resourceSpecAwsEc2Ssm,
} from 'teleport/Discover/Fixtures/fixtures';
import { SelectResourceSpec } from 'teleport/Discover/SelectResource/resources';
import { AgentMeta } from 'teleport/Discover/useDiscover';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { ConfigureDiscoveryService as Comp } from './ConfigureDiscoveryService';

export default {
  title: 'Teleport/Discover/Shared/ConfigureDiscoveryService',
};

export const Server = () => {
  return <Component resourceSpec={resourceSpecAwsEc2Ssm} />;
};

export const Database = () => {
  return <Component resourceSpec={resourceSpecAwsRdsPostgres} />;
};

export const WithCreateConfig = () => {
  return (
    <Component
      resourceSpec={resourceSpecAwsRdsPostgres}
      withCreateConfig={true}
    />
  );
};
WithCreateConfig.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.discoveryConfigPath, () => HttpResponse.json({})),
    ],
  },
};

export const WithCreateConfigFailed = () => {
  return (
    <Component
      resourceSpec={resourceSpecAwsRdsPostgres}
      withCreateConfig={true}
    />
  );
};
WithCreateConfigFailed.parameters = {
  msw: {
    handlers: [
      http.post(
        cfg.api.discoveryConfigPath,
        () =>
          HttpResponse.json(
            {
              message: 'Whoops, creating config error',
            },
            { status: 403 }
          ),
        { once: true }
      ),
      http.post(cfg.api.discoveryConfigPath, () => HttpResponse.json({})),
    ],
  },
};

const Component = ({
  resourceSpec,
  withCreateConfig = false,
}: {
  resourceSpec: SelectResourceSpec;
  withCreateConfig?: boolean;
}) => {
  const agentMeta: AgentMeta = {
    resourceName: 'aws-console',
    agentMatcherLabels: [],
    awsRegion: 'ap-south-1',
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
    autoDiscovery: {
      config: {
        name: 'discovery-config-name',
        discoveryGroup: 'discovery-group-name',
        aws: [],
      },
    },
  };
  return (
    <RequiredDiscoverProviders
      agentMeta={agentMeta}
      resourceSpec={resourceSpec}
    >
      {withCreateConfig && (
        <Info>Devs: Click next to see create config dialog</Info>
      )}
      <Comp withCreateConfig={withCreateConfig} />
    </RequiredDiscoverProviders>
  );
};
