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

import { Link, Text } from 'design';

import {
  ActionButtons,
  TextBox,
  PermissionsErrorMessage,
} from 'teleport/Discover/Shared';

export function ServerResource(props: ServerResourceProps) {
  let content = <TeleportVersions />;
  if (props.disabled) {
    content = (
      <PermissionsErrorMessage
        action="add new Servers"
        productName="Server Access"
      />
    );
  }

  return (
    <>
      {content}

      <ActionButtons
        onProceed={() => props.onProceed()}
        disableProceed={props.disabled}
      />
    </>
  );
}

function TeleportVersions() {
  return (
    <TextBox>
      <Text typography="h5">
        Teleport officially supports the following operating systems:
      </Text>
      <ul style={{ paddingLeft: 28 }}>
        <li>Ubuntu 14.04+</li>
        <li>Debian 8+</li>
        <li>RHEL/CentOS 7+</li>
        <li>Amazon Linux 2</li>
        <li>macOS (Intel)</li>
      </ul>
      <Text>
        For a more comprehensive list, visit{' '}
        <Link href="https://goteleport.com/download" target="_blank">
          https://goteleport.com/download
        </Link>
        .
      </Text>
    </TextBox>
  );
}

interface ServerResourceProps {
  disabled: boolean;
  onProceed: () => void;
}
