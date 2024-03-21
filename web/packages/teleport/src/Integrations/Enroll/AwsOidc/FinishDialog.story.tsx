/**
 * Copyright 2023 Gravitational, Inc.
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

import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { FinishDialog } from './FinishDialog';

export default {
  title: 'Teleport/Integrations/Enroll/AwsOidc',
};

export const Story = () => (
  <MemoryRouter>
    <FinishDialog
      integration={{
        resourceType: 'integration',
        kind: IntegrationKind.AwsOidc,
        name: 'some-integration-name',
        statusCode: IntegrationStatusCode.Running,
        spec: {
          roleArn: 'some-role-arn',
          issuerS3Bucket: '',
          issuerS3Prefix: '',
        },
      }}
    />
  </MemoryRouter>
);
Story.storyName = 'FinishDialogue';

export const FinishDialogueDiscover = () => (
  <MemoryRouter initialEntries={[{ state: { discover: {} } }]}>
    <FinishDialog
      integration={{
        resourceType: 'integration',
        kind: IntegrationKind.AwsOidc,
        name: 'some-integration-name',
        statusCode: IntegrationStatusCode.Running,
        spec: {
          roleArn: 'some-role-arn',
          issuerS3Bucket: '',
          issuerS3Prefix: '',
        },
      }}
    />
  </MemoryRouter>
);
