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

import React from 'react';
import { MemoryRouter } from 'react-router';
import { http, HttpResponse, delay } from 'msw';

import { Resource } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';

import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';

import {
  createTeleportContext,
  getAcl,
  noAccess,
} from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport';

import { UserContext } from 'teleport/User/UserContext';

import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import { IntegrationKind } from 'teleport/services/integrations';

import { Acl } from 'teleport/services/user';
import cfg from 'teleport/config';

import { IntegrationStatus } from '../IntegrationStatus';

const rawIntegrationResponse = {
  name: 'aws-integration',
  subKind: 'aws-oidc',
  awsoidc: { roleArn: 'role-arn' },
};

export default {
  title: 'Teleport/Integrations/Status/AwsOidc',
};

export const WithFullAccess = () => {
  return <Component />;
};

WithFullAccess.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.integrationsPath, () =>
        HttpResponse.json(rawIntegrationResponse)
      ),
    ],
  },
};

export const WithNoAccess = () => {
  const customAcl = getAcl({ noAccess: true });
  return <Component customAcl={customAcl} />;
};

export const WithPartialAccess = () => {
  const customAcl = getAcl();
  customAcl.dbServers = noAccess;
  return <Component customAcl={customAcl} />;
};

// export const WithCta = () => {
//   return <Component />;
// };

const Component = ({ customAcl }: { customAcl?: Acl }) => {
  const ctx = createTeleportContext({ customAcl: customAcl || getAcl() });
  return (
    <MemoryRouter
      initialEntries={[
        {
          pathname: cfg.getIntegrationStatusRoute(
            IntegrationKind.AwsOidc,
            'some-aws-oidc-name'
          ),
        },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <IntegrationStatus />
      </ContextProvider>
    </MemoryRouter>
  );
};
