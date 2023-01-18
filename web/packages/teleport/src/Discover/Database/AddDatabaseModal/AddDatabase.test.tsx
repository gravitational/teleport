/**
 * Copyright 2021 Gravitational, Inc.
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
import { render, screen } from 'design/utils/testing';

import {
  DatabaseLocation,
  DatabaseEngine,
} from 'teleport/Discover/Database/resources';

import { Props, AddDatabase } from './AddDatabase';
import { State } from './useAddDatabase';

describe('correct database add command generated with given input', () => {
  test.each`
    desc                     | location                       | engine                       | output
    ${'self-hosted mysql'}   | ${DatabaseLocation.SelfHosted} | ${DatabaseEngine.MySQL}      | ${'teleport db configure create --token=[generated-join-token] --proxy=localhost:443 --name=[db-name] --protocol=mysql --uri=[uri] -o file'}
    ${'rds mysql'}           | ${DatabaseLocation.AWS}        | ${DatabaseEngine.MySQL}      | ${'teleport db configure create --token=[generated-join-token] --proxy=localhost:443 --name=[db-name] --protocol=mysql --uri=[uri] -o file --aws-region=[region]'}
    ${'cloud sql mysql'}     | ${DatabaseLocation.GCP}        | ${DatabaseEngine.MySQL}      | ${'teleport db configure create --token=[generated-join-token] --proxy=localhost:443 --name=[db-name] --protocol=mysql --uri=[uri] -o file --ca-cert-file=[instance-ca-filepath] --gcp-project-id=[project-id] --gcp-instance-id=[instance-id]'}
    ${'rds postgres'}        | ${DatabaseLocation.AWS}        | ${DatabaseEngine.PostgreSQL} | ${'teleport db configure create --token=[generated-join-token] --proxy=localhost:443 --name=[db-name] --protocol=postgres --uri=[uri] -o file --aws-region=[region]'}
    ${'cloud sql postgres'}  | ${DatabaseLocation.GCP}        | ${DatabaseEngine.PostgreSQL} | ${'teleport db configure create --token=[generated-join-token] --proxy=localhost:443 --name=[db-name] --protocol=postgres --uri=[uri] -o file --ca-cert-file=[instance-ca-filepath] --gcp-project-id=[project-id] --gcp-instance-id=[instance-id]'}
    ${'redshift'}            | ${DatabaseLocation.AWS}        | ${DatabaseEngine.RedShift}   | ${'teleport db configure create --token=[generated-join-token] --proxy=localhost:443 --name=[db-name] --protocol=postgres --uri=[uri] -o file --aws-region=[region] --aws-redshift-cluster-id=[cluster-id]'}
    ${'self-hosted mongodb'} | ${DatabaseLocation.SelfHosted} | ${DatabaseEngine.Mongo}      | ${'teleport db configure create --token=[generated-join-token] --proxy=localhost:443 --name=[db-name] --protocol=mongodb --uri=[uri] -o file'}
  `(
    'should generate correct command for input: $desc',
    ({ location, engine, output }) => {
      render(
        <AddDatabase
          {...props}
          selectedDb={{ location, engine, name: 'n/a' }}
        />
      );

      expect(screen.getByText(output)).toBeInTheDocument();
    }
  );
});

const props: Props & State = {
  isEnterprise: false,
  username: 'yassine',
  version: '6.1.3',
  onClose: () => null,
  authType: 'local',
  attempt: {
    status: 'failed',
    statusText: '',
  } as any,
  token: { id: 'some-token', expiryText: '4 hours', expiry: null },
  createJoinToken() {
    return Promise.resolve(null);
  },
  selectedDb: null,
};
