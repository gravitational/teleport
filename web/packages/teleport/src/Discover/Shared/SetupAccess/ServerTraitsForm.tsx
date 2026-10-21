/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { useState } from 'react';

import { Box, Text } from 'design';

import {
  type Option,
  SelectCreatable,
} from 'teleport/Discover/Shared/SelectCreatable';

type ServerTraitsFormProps = {
  // Selected logins.
  logins: Option[];
  // Fixed refers to traits inherited from roles.
  // They cannot be modified/deleted by the user.
  fixedLogins?: Option[];
  // Selectables refer to dynamic traits already stored on the user.
  // User's can modify/delete them.
  selectableLogins?: Option[];

  disabled?: boolean;
  showAutoDiscoveryHint?: boolean;
  onLoginsChange(logins: Option[]): void;
};

export function ServerTraitsForm({
  logins,
  fixedLogins = [],
  selectableLogins = [],
  disabled = false,
  showAutoDiscoveryHint = false,
  onLoginsChange,
}: ServerTraitsFormProps) {
  const [loginInputValue, setLoginInputValue] = useState('');

  function handleLoginKeyDown(event: React.KeyboardEvent) {
    if (!loginInputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        onLoginsChange([
          ...logins,
          { value: loginInputValue, label: loginInputValue },
        ]);
        setLoginInputValue('');
        event.preventDefault();
    }
  }

  return (
    <>
      {showAutoDiscoveryHint && (
        <Text mb={3}>
          Since auto-discovery is enabled, make sure to include all OS users
          that will be used to connect to the discovered EC2 instances.
        </Text>
      )}
      <Box mb={2}>
        OS Users
        <SelectCreatable
          inputValue={loginInputValue}
          isClearable={logins.some(login => !login.isFixed)}
          onInputChange={setLoginInputValue}
          onKeyDown={handleLoginKeyDown}
          placeholder="Start typing OS users and press enter"
          value={logins}
          isDisabled={disabled}
          onChange={(value, action) => {
            onLoginsChange(
              action.action === 'clear' ? fixedLogins : value || []
            );
          }}
          options={selectableLogins}
          autoFocus
        />
      </Box>
    </>
  );
}
