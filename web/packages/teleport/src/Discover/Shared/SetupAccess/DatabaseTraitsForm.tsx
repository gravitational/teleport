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

type DatabaseTraitsFormProps = {
  // Selected db users.
  databaseUsers: Option[];
  // Selected db names.
  databaseNames: Option[];

  // Fixed refers to traits inherited from roles.
  // They cannot be modified/deleted by the user.
  fixedDatabaseUsers?: Option[];
  fixedDatabaseNames?: Option[];

  // Selectables refer to dynamic traits already stored on the user.
  // User's can modify/delete them.
  selectableDatabaseUsers?: Option[];
  selectableDatabaseNames?: Option[];

  disabled?: boolean;
  showAutoDiscoveryHint?: boolean;
  onDatabaseUsersChange(databaseUsers: Option[]): void;
  onDatabaseNamesChange(databaseNames: Option[]): void;
};

export function DatabaseTraitsForm({
  databaseUsers,
  databaseNames,
  fixedDatabaseUsers = [],
  fixedDatabaseNames = [],
  selectableDatabaseUsers = [],
  selectableDatabaseNames = [],
  disabled = false,
  showAutoDiscoveryHint = false,
  onDatabaseUsersChange,
  onDatabaseNamesChange,
}: DatabaseTraitsFormProps) {
  const [nameInputValue, setNameInputValue] = useState('');
  const [userInputValue, setUserInputValue] = useState('');

  function handleNameKeyDown(event: React.KeyboardEvent) {
    if (!nameInputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        onDatabaseNamesChange([
          ...databaseNames,
          { value: nameInputValue, label: nameInputValue },
        ]);
        setNameInputValue('');
        event.preventDefault();
    }
  }

  function handleUserKeyDown(event: React.KeyboardEvent) {
    if (!userInputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        onDatabaseUsersChange([
          ...databaseUsers,
          { value: userInputValue, label: userInputValue },
        ]);
        setUserInputValue('');
        event.preventDefault();
    }
  }

  return (
    <>
      {showAutoDiscoveryHint && (
        <Text mb={3}>
          Since auto-discovery is enabled, make sure to include all database
          users and names that will be used to connect to the discovered
          databases.
        </Text>
      )}
      <Box mb={4}>
        Database Users
        <SelectCreatable
          inputValue={userInputValue}
          isClearable={databaseUsers.some(v => !v.isFixed)}
          onInputChange={setUserInputValue}
          onKeyDown={handleUserKeyDown}
          placeholder="Start typing database users and press enter"
          value={databaseUsers}
          isDisabled={disabled}
          onChange={(value, action) => {
            onDatabaseUsersChange(
              action.action === 'clear' ? fixedDatabaseUsers : value || []
            );
          }}
          options={selectableDatabaseUsers}
          autoFocus
        />
      </Box>
      <Box mb={2}>
        Database Names
        <SelectCreatable
          inputValue={nameInputValue}
          isClearable={databaseNames.some(v => !v.isFixed)}
          onInputChange={setNameInputValue}
          onKeyDown={handleNameKeyDown}
          placeholder="Start typing database names and press enter"
          value={databaseNames}
          isDisabled={disabled}
          onChange={(value, action) => {
            onDatabaseNamesChange(
              action.action === 'clear' ? fixedDatabaseNames : value || []
            );
          }}
          options={selectableDatabaseNames}
        />
      </Box>
    </>
  );
}
