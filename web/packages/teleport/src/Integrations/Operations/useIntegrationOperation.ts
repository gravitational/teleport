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
import { useState } from 'react';

import { integrationService } from 'teleport/services/integrations';

import type { Integration, Plugin } from 'teleport/services/integrations';

export function useIntegrationOperation() {
  const [operation, setOperation] = useState({
    type: 'none',
  } as Operation);

  function clear() {
    setOperation({ type: 'none' });
  }

  function remove() {
    return integrationService.deleteIntegration(operation.item.name);
  }

  function edit(req: EditableIntegrationFields) {
    return integrationService.updateIntegration(operation.item.name, {
      awsoidc: {
        roleArn: req.roleArn,
        issuerS3Bucket: req.s3Bucket,
        issuerS3Prefix: req.s3Prefix,
      },
    });
  }

  function onRemove(item: Integration) {
    setOperation({ type: 'delete', item });
  }

  function onEdit(item: Integration) {
    setOperation({ type: 'edit', item });
  }

  return {
    ...operation,
    clear,
    remove,
    edit,
    onRemove,
    onEdit,
  };
}

/**
 * Currently only integration updateable is aws oidc.
 */
export type EditableIntegrationFields = {
  roleArn: string;
  s3Bucket: string;
  s3Prefix: string;
};

export type OperationType = 'create' | 'edit' | 'delete' | 'reset' | 'none';

export type ExternalAuditStorageOpType = 'draft' | 'cluster';

export type Operation = {
  type: OperationType;
  item?: Plugin | Integration | { name: ExternalAuditStorageOpType };
};
