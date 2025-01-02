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

import { Integration } from 'teleport/services/integrations';

import { EditAwsOidcIntegrationDialog } from '../EditAwsOidcIntegrationDialog';
import { DeleteIntegrationDialog } from '../RemoveIntegrationDialog';
import {
  EditableIntegrationFields,
  OperationType,
} from './useIntegrationOperation';

type Props = {
  operation: OperationType;
  integration: Integration;
  close(): void;
  edit(integration: Integration, req: EditableIntegrationFields): Promise<void>;
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
      <EditAwsOidcIntegrationDialog
        integration={integration}
        close={close}
        edit={edit}
      />
    );
  }

  return null;
}
