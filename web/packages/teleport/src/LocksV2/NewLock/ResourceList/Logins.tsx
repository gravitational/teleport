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

import React, { useState } from 'react';
import Table from 'design/DataTable';
import { Box, ButtonSecondary, Flex, Input, Text } from 'design';

import { renderActionCell, LoginsProps } from './common';

export function Logins(props: LoginsProps) {
  const [loginInput, setLoginInput] = useState('');
  const [addedLogins, setAddedLogins] = useState<{ login: string }[]>(() => {
    const loginMap = props.selectedResources.login;
    return Object.keys(loginMap).map(login => ({ login }));
  });

  function addLogin(e: React.MouseEvent<HTMLButtonElement>) {
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
            !loginInput.length || props.selectedResources['login'][loginInput]
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
