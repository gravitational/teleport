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
};

export type OperationType = 'create' | 'edit' | 'delete' | 'reset' | 'none';

export type ExternalAuditStorageOpType = 'draft' | 'cluster';

export type Operation = {
  type: OperationType;
  item?: Plugin | Integration | { name: ExternalAuditStorageOpType };
};
