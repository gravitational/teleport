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

import { useState } from 'react';

import {
  IntegrationAwsOidc,
  IntegrationKind,
  integrationService,
  type Integration,
} from 'teleport/services/integrations';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { DeleteRequestOptions } from './IntegrationOperations';

export function useIntegrationOperation() {
  const { clusterId } = useStickyClusterId();

  const [operation, setOperation] = useState<Operation>({
    type: 'none',
  });

  function clear() {
    setOperation({ type: 'none' });
  }

  function remove(opt: DeleteRequestOptions = {}) {
    return integrationService.deleteIntegration({
      name: operation.item.name,
      clusterId,
      associatedResources: opt.deleteAssociatedResources,
    });
  }

  async function edit(req: EditableIntegrationFields) {
    // Health check with the new roleArn to validate that
    // connection still works.
    if (req.kind === IntegrationKind.AwsOidc) {
      try {
        await integrationService.pingAwsOidcIntegration(
          {
            integrationName: operation.item.name,
            clusterId,
          },
          { roleArn: req.roleArn }
        );
        return integrationService.updateIntegration(operation.item.name, {
          kind: IntegrationKind.AwsOidc,
          awsoidc: {
            roleArn: req.roleArn,
          },
        });
      } catch (err) {
        throw new Error(`Health check failed: ${err}`);
      }
    }
  }

  function onRemove(item: Integration) {
    setOperation({ type: 'delete', item });
  }

  function onEdit(item: EditableIntegration) {
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

export type AwsOidcIntegrationEditableFields = {
  kind: IntegrationKind.AwsOidc;
  roleArn: string;
};

export type EditableIntegrationFields = AwsOidcIntegrationEditableFields;

export type OperationType = 'create' | 'edit' | 'delete' | 'reset' | 'none';

export type ExternalAuditStorageOpType = 'draft' | 'cluster';

export type Operation = {
  type: OperationType;
  item?: Integration;
};

export type EditableIntegration = IntegrationAwsOidc;
