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

import cfg from 'teleport/config';
import { resourceSpecAwsRdsPostgres } from 'teleport/Discover/Fixtures/databases';
import { RequiredDiscoverProviders } from 'teleport/Discover/Fixtures/fixtures';
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
      http.get(cfg.getIntegrationsUrl(), () =>
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

  return (
    <RequiredDiscoverProviders
      agentMeta={{}}
      resourceSpec={resourceSpecAwsRdsPostgres}
      customAcl={noAccess ? getAcl({ noAccess }) : undefined}
    >
      <AwsAccount />
    </RequiredDiscoverProviders>
  );
};
