/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import { Box, ButtonPrimary, ButtonSecondary, Text } from 'design';
import React, { useEffect, useState } from 'react';

import Validation, { Validator } from 'shared/components/Validation';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import FieldInput from 'shared/components/FieldInput';
import styled from 'styled-components';
import useAttempt from 'shared/hooks/useAttemptNext';

import { Header } from 'teleport/Discover/Shared';

type Line = {
  text: string;
  comment?: string;
};
export function ConfigureAccess({
  resourceService,
  organizationName,
  onNext,
  }) {
  // TODO set role
  const [roleName, setRoleName] = useState('github-' + organizationName);
  const [roleText, setRoleText] = useState('');
  const [lines, setLines] = useState<Line[]>([]);
  const { attempt, run } = useAttempt('');

  useEffect(() => {
    const roleText =
      `kind: role
metadata:
  name: ` + roleName + `
spec:
  allow:
    github_permissions:
    - orgs:
      - ` + organizationName + `
version: v7
`;
    setRoleText(roleText);
    setLines([{text:roleText}])
  }, [organizationName, roleName]);

  function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    run(() =>
      resourceService
        .createRole(roleText)
        .then(() => {
          onNext();
        })
    );
  }

  return (
    <Box pt={3}>
      <Header>Create a GitHub Integration</Header>
      <Box width="800px" mb={4}>
        Teleport can issue short-lived SSH certificates for GitHub access.
      </Box>

      <Container mb={5}>
        <Box width="600px" mb={4}>
          <Text bold>Step 5 Configure access to the Git server</Text>
          <Text>The Git server resource is successfully created.</Text>
          <br />
          <Text>
            To grant access to the Git server, create the following Teleport
            role for GitHub organization "{organizationName}".
          </Text>
        </Box>
        <Validation>
          {({ validator }) => (
            <>
              <FieldInput
                autoFocus={true}
                value={roleName}
                label="Enter Teleport role name:"
                onChange={e => setRoleName(e.target.value)}
              />
              <Text>Role definition:</Text>
              <TextSelectCopyMulti
                lines={lines}
                bash={false}
              />
              <Text>Once created, assign the role to desired Teleport users.</Text>
              <Box mt={6}>
                <ButtonPrimary
                  onClick={() => handleOnCreate(validator)}
                  disabled={
                    attempt.status === 'processing'
                  }
                >
                  Create Role
                </ButtonPrimary>
                <ButtonSecondary
                  onClick={() => onNext()}
                >
                  Skip
                </ButtonSecondary>
              </Box>
            </>
          )}
        </Validation>
      </Container>
    </Box>
  );
}

const Container = styled(Box)`
  max-width: 1000px;
  background-color: ${p => p.theme.colors.spotBackground[0]};
  border-radius: ${p => `${p.theme.space[2]}px`};
  padding: ${p => p.theme.space[3]}px;
`;
