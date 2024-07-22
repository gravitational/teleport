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

import React from 'react';

import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { IntegrationList } from './IntegrationList';
import { DeleteIntegrationDialog } from './RemoveIntegrationDialog';
import { EditAwsOidcIntegrationDialog } from './EditAwsOidcIntegrationDialog';
import { plugins, integrations } from './fixtures';

export default {
  title: 'Teleport/Integrations',
};

export function List() {
  return <IntegrationList list={[...plugins, ...integrations]} />;
}

export function DeleteDialog() {
  return (
    <DeleteIntegrationDialog
      close={() => null}
      remove={() => null}
      name="some-integration-name"
    />
  );
}

export function EditDialogWithoutS3() {
  return (
    <EditAwsOidcIntegrationDialog
      close={() => null}
      edit={() => null}
      integration={{
        resourceType: 'integration',
        kind: IntegrationKind.AwsOidc,
        name: 'some-integration-name',
        spec: {
          roleArn: 'arn:aws:iam::123456789012:role/johndoe',
        },
        statusCode: IntegrationStatusCode.Running,
      }}
    />
  );
}

export function EditDialogWithS3() {
  return (
    <EditAwsOidcIntegrationDialog
      close={() => null}
      edit={() => null}
      integration={{
        resourceType: 'integration',
        kind: IntegrationKind.AwsOidc,
        name: 'some-integration-name',
        spec: {
          roleArn: 'arn:aws:iam::123456789012:role/johndoe',
          issuerS3Bucket: 'named-bucket',
          issuerS3Prefix: 'named-prefix',
        },
        statusCode: IntegrationStatusCode.Running,
      }}
    />
  );
}
