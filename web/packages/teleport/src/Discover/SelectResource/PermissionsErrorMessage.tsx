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

import { Box, Text } from 'design';

import { ResourceKind } from '../Shared';
import { ResourceSpec } from './types';

export function PermissionsErrorMessage({
  resource,
}: PermissionsErrorMessageProps) {
  let action = 'add new';
  let productName = '';

  switch (resource.kind) {
    case ResourceKind.Application:
      action = `${action} Applications`;
      productName = 'Access Application';
      break;
    case ResourceKind.Database:
      action = `${action} Databases`;
      productName = 'Access Database';

      break;
    case ResourceKind.Desktop:
      action = `${action} Desktops`;
      productName = 'Access Desktop';

      break;
    case ResourceKind.Kubernetes:
      action = `${action} Kubernetes resources`;
      productName = 'Access Kubernetes';

      break;
    case ResourceKind.Server:
      action = `${action} Servers`;
      productName = 'Access Server';

      break;
    default:
      action = `${action} ${resource.name}s`;
      productName = `adding ${resource.name}s`;
  }

  return (
    <Box>
      <Text>
        You are not able to {action}. There are two possible reasons for this:
      </Text>
      <ul style={{ paddingLeft: 16, marginBottom: 2, marginTop: 2 }}>
        <li>
          Your Teleport Enterprise license does not include {productName}. Reach
          out to your Teleport administrator to enable {productName}.
        </li>
        <li>
          You don’t have sufficient permissions to {action}. Reach out to your
          Teleport administrator to request additional permissions.
        </li>
      </ul>
    </Box>
  );
}

interface PermissionsErrorMessageProps {
  resource: ResourceSpec;
}
