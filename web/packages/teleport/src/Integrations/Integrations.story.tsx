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

import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { IntegrationList } from './IntegrationList';
import { DeleteIntegrationDialog } from './RemoveIntegrationDialog';
import { EditAwsOidcIntegrationDialog } from './EditAwsOidcIntegrationDialog';
import { UpdateAwsOidcThumbprint } from './UpdateAwsOidcThumbprint';
import { plugins, integrations } from './fixtures';

export default {
  title: 'Teleport/Integrations',
};

export function List() {
  return <IntegrationList list={[...plugins, ...integrations]} />;
}

export function UpdateAwsOidcThumbprintHoverTooltip() {
  return (
    <UpdateAwsOidcThumbprint
      integration={{
        resourceType: 'integration',
        name: 'aws',
        kind: IntegrationKind.AwsOidc,
        statusCode: IntegrationStatusCode.Running,
        spec: { roleArn: '', issuerS3Prefix: '', issuerS3Bucket: '' },
      }}
    />
  );
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
          issuerS3Bucket: '',
          issuerS3Prefix: '',
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
