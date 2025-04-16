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

import React, { useState } from 'react';

import { Box, ButtonSecondary, Flex, Input, Text } from 'design';
import Table from 'design/DataTable';

import { LoginsProps, renderActionCell } from './common';

export function Logins(props: LoginsProps) {
  const [loginInput, setLoginInput] = useState('');
  const [addedLogins, setAddedLogins] = useState<{ login: string }[]>(() => {
    const loginMap = props.selectedResources.login;
    return Object.keys(loginMap).map(login => ({ login }));
  });

  function addLogin(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault(); // from form submit event

    props.toggleSelectResource({ kind: 'login', targetValue: loginInput });
    setLoginInput('');

    setAddedLogins([...addedLogins, { login: loginInput }]);
  }

  return (
    <Box>
      <Text mb={3}>
        Listing logins are not supported. Use the input box below to manually
        define a login to lock. <br />
        Double check the spelling before adding.
      </Text>
      <Flex
        alignItems="center"
        css={{ columnGap: '20px' }}
        mb={4}
        as="form"
        onSubmit={addLogin}
      >
        <Input
          autoFocus
          placeholder={`Enter a login name to lock`}
          width={500}
          value={loginInput}
          onChange={e => setLoginInput(e.currentTarget.value)}
        />
        <ButtonSecondary
          type="submit"
          size="large"
          disabled={
            !loginInput.length || !!props.selectedResources['login'][loginInput]
          }
        >
          + Add Login
        </ButtonSecondary>
      </Flex>
      <Table
        data={addedLogins}
        columns={[
          {
            key: 'login',
            headerText: 'Login',
          },
          {
            altKey: 'action-btn',
            render: ({ login }) =>
              renderActionCell(
                Boolean(props.selectedResources.login[login]),
                () =>
                  props.toggleSelectResource({
                    kind: 'login',
                    targetValue: login,
                  })
              ),
          },
        ]}
        emptyText="No Logins Added Yet"
        pagination={{ pageSize: props.pageSize }}
        isSearchable
      />
    </Box>
  );
}
