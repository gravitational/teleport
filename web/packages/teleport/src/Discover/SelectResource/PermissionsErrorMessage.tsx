/**
 * Copyright 2022 Gravitational, Inc.
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
import { Text, Box } from 'design';

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
