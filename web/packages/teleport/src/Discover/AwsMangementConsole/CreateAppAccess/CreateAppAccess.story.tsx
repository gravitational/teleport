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

import { Info } from 'design/Alert';

import cfg from 'teleport/config';
import {
  RequiredDiscoverProviders,
  resourceSpecAppAwsCliConsole,
} from 'teleport/Discover/Fixtures/fixtures';
import { AgentMeta } from 'teleport/Discover/useDiscover';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { CreateAppAccess } from './CreateAppAccess';

export default {
  title: 'Teleport/Discover/Application/AwsConsole/CreateApp',
};

export const Success = () => <Component />;
Success.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsAppAccess.createV2, () =>
        HttpResponse.json({ name: 'app-1' })
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
      http.post(
        cfg.api.awsAppAccess.createV2,
        async () => await delay('infinite')
      ),
    ],
  },
};

export const Failed = () => <Component />;
Failed.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsAppAccess.createV2, () =>
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

const Component = () => {
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
  };

  return (
    <RequiredDiscoverProviders
      agentMeta={agentMeta}
      resourceSpec={resourceSpecAppAwsCliConsole}
    >
      <Info>Devs: Click next to see next state</Info>
      <CreateAppAccess />
    </RequiredDiscoverProviders>
  );
};
