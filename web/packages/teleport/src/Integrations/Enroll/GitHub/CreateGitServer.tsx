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

import { Box, ButtonPrimary, Flex, Text } from 'design';
import React from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import Validation, { Validator } from 'shared/components/Validation';
import * as Icons from 'design/Icon';

import { Header } from 'teleport/Discover/Shared';
import { gitServerService } from 'teleport/services/gitservers';
import styled from 'styled-components';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';

export function CreateGitServer({
  organizationName,
  integrationName,
  onCreatedGitServer,
  }){

  const gitServerText =
    `kind: git_server
sub_kind: github
version: v2
spec:
  github:
    integration: ` + integrationName + `
    organization: ` + organizationName;
  const { attempt, run } = useAttempt('');

  function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    run(() =>
      gitServerService
        .createGitServer({
          subKind: "github",
          github: {
            organization: organizationName,
            integration: integrationName,
          },
        })
        .then(res =>{
          onCreatedGitServer(res);
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
          <Text bold>Step 4 Create a Git proxy server</Text>
          <Text>
            Now let's create a Git server resource for your Teleport users to proxy their git commands.
          </Text>
          <TextSelectCopyMulti
            lines={[{text:gitServerText}]}
            bash={false}
          />
        </Box>
        <Validation>
          {({ validator }) => (
            <>
              {attempt.status === 'failed' && (
                <Flex>
                  <Icons.Warning mr={2} color="error.main" size="small" />
                  <Text color="error.main">Error: {attempt.statusText}</Text>
                </Flex>
              )}
              <Box mt={6}>
                <ButtonPrimary
                  onClick={() => handleOnCreate(validator)}
                  disabled={attempt.status === 'success'}
                >
                  Create Git Server
                </ButtonPrimary>
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

