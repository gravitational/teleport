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

import React, { useState, useEffect } from 'react';
import { Box } from 'design';

import {
  SelectCreatable,
  Option,
} from 'teleport/Discover/Shared/SelectCreatable';
import {
  useUserTraits,
  SetupAccessWrapper,
} from 'teleport/Discover/Shared/SetupAccess';

import type { AgentStepProps } from '../../types';
import type { State } from 'teleport/Discover/Shared/SetupAccess';

export default function Container(props: AgentStepProps) {
  const state = useUserTraits(props);
  return <SetupAccess {...state} />;
}

export function SetupAccess(props: State) {
  const {
    onProceed,
    initSelectedOptions,
    getFixedOptions,
    getSelectableOptions,
    ...restOfProps
  } = props;
  const [loginInputValue, setLoginInputValue] = useState('');
  const [selectedLogins, setSelectedLogins] = useState<Option[]>([]);

  useEffect(() => {
    if (props.attempt.status === 'success') {
      setSelectedLogins(initSelectedOptions('logins'));
    }
  }, [props.attempt.status, initSelectedOptions]);

  function handleOnProceed() {
    onProceed({ logins: selectedLogins });
  }

  function handleLoginKeyDown(event: React.KeyboardEvent) {
    if (!loginInputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        setSelectedLogins([
          ...selectedLogins,
          { value: loginInputValue, label: loginInputValue },
        ]);
        setLoginInputValue('');
        event.preventDefault();
    }
  }

  const hasTraits = selectedLogins.length > 0;
  const canAddTraits = !props.isSsoUser && props.canEditUser;
  const headerSubtitle =
    'Select or create the OS users you will use to connect to server.';

  return (
    <SetupAccessWrapper
      {...restOfProps}
      headerSubtitle={headerSubtitle}
      traitKind="OS"
      traitDescription="users"
      hasTraits={hasTraits}
      onProceed={handleOnProceed}
    >
      <Box mb={2}>
        OS Users
        <SelectCreatable
          inputValue={loginInputValue}
          isClearable={selectedLogins.some(v => !v.isFixed)}
          onInputChange={setLoginInputValue}
          onKeyDown={handleLoginKeyDown}
          placeholder="Start typing OS users and press enter"
          value={selectedLogins}
          isDisabled={!canAddTraits}
          onChange={(value, action) => {
            if (action.action === 'clear') {
              setSelectedLogins(getFixedOptions('logins'));
            } else {
              setSelectedLogins(value || []);
            }
          }}
          options={getSelectableOptions('logins')}
        />
      </Box>
    </SetupAccessWrapper>
  );
}
