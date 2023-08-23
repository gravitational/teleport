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

import { Integration } from 'teleport/services/integrations';

import { DeleteIntegrationDialog } from '../RemoveIntegrationDialog';
import { EditIntegrationDialog } from '../EditIntegrationDialog';

import {
  OperationType,
  EditableIntegrationFields,
} from './useIntegrationOperation';

type Props = {
  operation: OperationType;
  integration: Integration;
  close(): void;
  edit(req: EditableIntegrationFields): Promise<void>;
  remove(): Promise<void>;
};

export function IntegrationOperations({
  operation,
  integration,
  close,
  edit,
  remove,
}: Props) {
  if (operation === 'delete') {
    return (
      <DeleteIntegrationDialog
        name={integration.name}
        close={close}
        remove={remove}
      />
    );
  }

  if (operation === 'edit') {
    return (
      <EditIntegrationDialog
        integration={integration}
        close={close}
        edit={edit}
      />
    );
  }

  return null;
}
