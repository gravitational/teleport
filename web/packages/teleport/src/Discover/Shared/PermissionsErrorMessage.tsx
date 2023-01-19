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

import { Text } from 'design';

import { TextBox } from 'teleport/Discover/Shared';

export function PermissionsErrorMessage(props: PermissionsErrorMessageProps) {
  return (
    <TextBox>
      <Text typography="h5">
        You are not able to {props.action}. There are two possible reasons for
        this:
      </Text>
      <ul style={{ paddingLeft: 28 }}>
        <li>
          Your Teleport Enterprise license does not include {props.productName}.
          Reach out to your Teleport administrator to enable {props.productName}
          .
        </li>
        <li>
          You don’t have sufficient permissions to {props.action}. Reach out to
          your Teleport administrator to request additional permissions.
        </li>
      </ul>
    </TextBox>
  );
}

interface PermissionsErrorMessageProps {
  action: string;
  productName: string;
}
