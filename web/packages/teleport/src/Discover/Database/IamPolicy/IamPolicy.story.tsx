/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
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
