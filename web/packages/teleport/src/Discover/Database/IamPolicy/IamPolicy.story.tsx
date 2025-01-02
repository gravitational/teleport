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

import { MemoryRouter } from 'react-router';

import { IamPolicyView } from './IamPolicy';
import type { State } from './useIamPolicy';

export default {
  title: 'Teleport/Discover/Database/IamPolicy',
};

export const Loaded = () => (
  <MemoryRouter>
    <IamPolicyView {...props} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <IamPolicyView
      {...props}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  </MemoryRouter>
);

export const Processing = () => (
  <MemoryRouter>
    <IamPolicyView {...props} attempt={{ status: 'processing' }} />
  </MemoryRouter>
);

const props: State = {
  attempt: {
    status: 'success',
    statusText: '',
  },
  nextStep: () => null,
  fetchIamPolicy: () => null,
  iamPolicy: {
    type: 'aws',
    aws: {
      placeholders: '',
      policy_document:
        '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"redshift:GetClusterCredentials","Resource":["arn:aws:redshift:us-east-1:12345:dbuser:redshift-cluster-1/*","arn:aws:redshift:us-east-1:12345:dbname:redshift-cluster-1/*","arn:aws:redshift:us-east-1:12345:dbgroup:redshift-cluster-1/*"]}]}',
    },
  },
  iamPolicyName: 'TeleportPolicyName',
};
