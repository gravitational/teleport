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
import { Box, ButtonPrimary, ButtonSecondary, Flex, Text } from 'design';
import { Link } from 'react-router-dom';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import React from 'react';
import styled from 'styled-components';

import cfg from 'teleport/config';
import { Header } from 'teleport/Discover/Shared';

export function Usage({
  integration,

                      }){
  return(
  <Box pt={3}>
    <Header>Create a GitHub Integration</Header>

    <Box width="800px" mb={4}>
      Teleport can issue short-lived SSH certificates for GitHub access.
    </Box>

    <Box width="800px" mb={4}>
      <Container mb={5}>
        <Box mb={4}>
          <Text>
            You are done!
          </Text>
          <br/>
          <Text>
            Once your Teleport user is granted permission to the Git server, login using "tsh".
          </Text>

          <br/>
          <Text> To list the Git servers: </Text>
          <TextSelectCopy
            mt="2"
            text={`tsh git ls`}
          />
          {'To clone a new repository, find the SSH url of the repository on '}
          <Link href={ `https://github.com/organizations/${integration.spec.organization}` } target="_blank" >
            github.com
          </Link>
          {' then'}
          <TextSelectCopy
            mt="2"
            text={`tsh git clone <git-clone-ssh-url>`}
          />
          {'To configure an existing git repository, go to the repository then:'}
          <TextSelectCopy
            mt="2"
            text={`tsh git config update`}
          />
          <Text>
            Once the repository is cloned or configured, use 'git' as normal.
          </Text>
          <br/>
          <ButtonPrimary as={Link} to={cfg.routes.integrations} size="large">
            Go to Integration List
          </ButtonPrimary>
        </Box>
      </Container>
    </Box>
  </Box>
  )
}

const Container = styled(Box)`
  max-width: 1000px;
  background-color: ${p => p.theme.colors.spotBackground[0]};
  border-radius: ${p => `${p.theme.space[2]}px`};
  padding: ${p => p.theme.space[3]}px;
`;
