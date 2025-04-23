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
