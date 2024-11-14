/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { LabelKind } from 'design/Label/Label';

import {
  getStatusCodeTitle,
  Integration,
} from 'teleport/services/integrations';
import { getStatus, Status } from 'teleport/Integrations/IntegrationList';

export function getStatusAndLabel(integration?: Integration): {
  labelKind: LabelKind;
  status: string;
} {
  if (!integration) {
    return;
  }
  const modifiedStatus = getStatus(integration);
  const statusCode = integration.statusCode;
  switch (modifiedStatus) {
    case Status.Success: {
      return { labelKind: 'success', status: getStatusCodeTitle(statusCode) };
    }
    case Status.Error: {
      return { labelKind: 'danger', status: getStatusCodeTitle(statusCode) };
    }
    case Status.Warning: {
      return { labelKind: 'warning', status: getStatusCodeTitle(statusCode) };
    }
    default: // unknown
      return { labelKind: 'secondary', status: getStatusCodeTitle(statusCode) };
  }
}
