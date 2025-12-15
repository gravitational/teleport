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

import React, { useEffect, useState } from 'react';

import { Box, Text } from 'design';

import {
  Option,
  SelectCreatable,
} from 'teleport/Discover/Shared/SelectCreatable';
import {
  SetupAccessWrapper,
  useUserTraits,
  type State,
} from 'teleport/Discover/Shared/SetupAccess';

export default function Container() {
  const state = useUserTraits();
  return <SetupAccess {...state} />;
}

export function SetupAccess(props: State) {
  const {
    onProceed,
    initSelectedOptions,
    getFixedOptions,
    getSelectableOptions,
    agentMeta,
    ...restOfProps
  } = props;
  const [loginInputValue, setLoginInputValue] = useState('');
  const [selectedLogins, setSelectedLogins] = useState<Option[]>([]);

  const wantAutoDiscover = !!agentMeta.autoDiscovery;

  useEffect(() => {
    if (props.attempt.status === 'success') {
      setSelectedLogins(initSelectedOptions('logins'));
    }
  }, [props.attempt.status, initSelectedOptions]);

  function handleOnProceed() {
    let numStepsToIncrement;
    // Skip test connection since test connection currently
    // only supports one resource testing and auto enrolling
    // enrolls resources > 1.
    if (wantAutoDiscover) {
      numStepsToIncrement = 2;
    }
    onProceed({ logins: selectedLogins }, numStepsToIncrement);
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
      wantAutoDiscover={wantAutoDiscover}
    >
      {wantAutoDiscover && (
        <Text mb={3}>
          Since auto-discovery is enabled, make sure to include all OS users
          that will be used to connect to the discovered EC2 instances.
        </Text>
      )}
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
