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
import { EditIntegrationDialog } from './EditIntegrationDialog';
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

export function EditDialog() {
  return (
    <EditIntegrationDialog
      close={() => null}
      edit={() => null}
      integration={{
        resourceType: 'integration',
        kind: IntegrationKind.AwsOidc,
        name: 'some-integration-name',
        spec: { roleArn: 'arn:aws:iam::123456789012:roles/johndoe' },
        statusCode: IntegrationStatusCode.Running,
      }}
    />
  );
}
