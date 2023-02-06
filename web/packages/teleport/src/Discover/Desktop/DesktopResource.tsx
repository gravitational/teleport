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

import { InfoFilled } from 'design/Icon';

import {
  ActionButtons,
  TextBox,
  PermissionsErrorMessage,
} from 'teleport/Discover/Shared';

export function DesktopResource(props: DesktopResourceProps) {
  let content = (
    <TextBox>
      <Text typography="h5" bold mb="4px">
        <InfoFilled mr="8px" fontSize="14px" />
        Note
      </Text>
      <Text>
        Teleport Desktop Access currently only supports Windows Desktops managed
        by Active Directory (AD).
      </Text>
      <Text>We are working on adding support for non-AD Windows Desktops.</Text>
    </TextBox>
  );

  if (props.disabled) {
    content = (
      <PermissionsErrorMessage
        action="add new Desktops"
        productName="Desktop Access"
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

interface DesktopResourceProps {
  disabled: boolean;
  onProceed: () => void;
}
