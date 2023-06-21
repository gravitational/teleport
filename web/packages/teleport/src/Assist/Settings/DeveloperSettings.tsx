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

import { CheckboxInput, CheckboxWrapper } from 'design/Checkbox';

import { Description, Title } from 'teleport/Assist/Settings/shared';

interface DeveloperSettingsProps {
  debugMenuEnabled: boolean;
  onDebugMenuToggle: (enabled: boolean) => void;
}

export function DeveloperSettings(props: DeveloperSettingsProps) {
  return (
    <div>
      <Title>Debug menu</Title>

      <Description>
        The debug menu allows you to add items to the Assist UI for development
        purposes.
      </Description>

      <CheckboxWrapper
        as="label"
        htmlFor="setDefault"
        style={{ border: 'none', padding: '2px 0' }}
      >
        <CheckboxInput
          type="checkbox"
          name="Make Default Payment"
          id="setDefault"
          data-testid="set-default"
          onChange={e => {
            props.onDebugMenuToggle(e.target.checked);
          }}
          checked={props.debugMenuEnabled}
        />
        Enable the debug menu
      </CheckboxWrapper>
    </div>
  );
}
