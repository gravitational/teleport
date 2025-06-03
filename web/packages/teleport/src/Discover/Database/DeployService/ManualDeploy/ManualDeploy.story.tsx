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

import cfg from 'teleport/config';
import { resourceSpecAwsRdsPostgres } from 'teleport/Discover/Fixtures/databases';
import { RequiredDiscoverProviders } from 'teleport/Discover/Fixtures/fixtures';
import { INTERNAL_RESOURCE_ID_LABEL_KEY } from 'teleport/services/joinToken';

import ManualDeploy from './ManualDeploy';

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
        HttpResponse.json(rawJoinToken)
      ),
    ],
  },
};

const Provider = props => {
  return (
    <RequiredDiscoverProviders
      agentMeta={{
        resourceName: 'db-name',
        agentMatcherLabels: [],
        db: {} as any,
        selectedAwsRdsDb: {} as any,
        ...props.agentMeta,
      }}
      resourceSpec={resourceSpecAwsRdsPostgres}
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
