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
import { Box, ButtonPrimary, Link, Text } from 'design';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import React from 'react';
import styled from 'styled-components';

import { Header } from 'teleport/Discover/Shared';

export function ConfigureGitHub({
  integration,
  onNext,
  }){
  return (
    <Box pt={3}>
      <Header>Create a GitHub Integration</Header>
      <Box width="800px" mb={4}>
        Teleport can issue short-lived SSH certificates for GitHub access.
      </Box>
      <Container mb={5}>
        <Box width="600px">
          <Text bold>Step 3 Configure SSH certificate authority on GitHub</Text>
          <Text>
            GitHub integration "{integration.name}" successfully added. Now we want add the SSH certificate authority to your GitHub organization.
          </Text>
          <br/>
          <Text>
            Go to your organization's {' '}
            <Link
              target="_blank"
              href={`https://github.com/organizations/${integration.spec.organization}/settings/security`}
            >
              Authentication security page
            </Link> and click on "New CA":
          </Text>
          <TextSelectCopy bash={false} text={integration.spec.publicKeys[0]}>
          </TextSelectCopy>
          <br />
          <Text>
            After the CA is added, it should have the following SHA256 fingerprint:
          </Text>
          <TextSelectCopy bash={false} text={integration.spec.fingerprints[0].replace("SHA256:","")}>
          </TextSelectCopy>
        </Box>
      </Container>
      <Box mt={6}>
        <ButtonPrimary
          onClick={() => onNext()}
        >
          Next
        </ButtonPrimary>
      </Box>
    </Box>
  );
}
const Container = styled(Box)`
  max-width: 1000px;
  background-color: ${p => p.theme.colors.spotBackground[0]};
  border-radius: ${p => `${p.theme.space[2]}px`};
  padding: ${p => p.theme.space[3]}px;
`;

