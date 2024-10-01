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
import Validation, { Validator } from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import useAttempt from 'shared/hooks/useAttemptNext';
import * as Icons from 'design/Icon';
import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import {
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import { Header } from 'teleport/Discover/Shared';

export function CreateIntegration({
  spec,
  onCreatedIntegration,
  }){
  const [integrationName, setIntegrationName] = useState('');
  const [isIntegrationNameEdited, setIsIntegrationNameEdited] = useState(false);
  const { attempt, run } = useAttempt('');

  useEffect(() => {
    if (!isIntegrationNameEdited) {
      setIntegrationName("github-" + spec.organization);
    }
  }, [spec.organization, isIntegrationNameEdited]);

  function handleIntegrationNameChange(value: string) {
    setIntegrationName(value);
    setIsIntegrationNameEdited(true);
  }

  function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    run(() =>
      integrationService
        .createIntegration({
          name: integrationName,
          subKind: IntegrationKind.GitHub,
          github: spec,
        })
        .then(res => {
          onCreatedIntegration(res);
        })
    );
  }

  return (
    <Box pt={3}>
      <Header>Create a GitHub Integration</Header>

      <Box width="800px" mb={4}>
        Teleport can issue short-lived SSH certificates for GitHub access.
      </Box>

      <Validation>
        {({ validator }) => (
          <>
            <Container mb={5}>
              <Text bold>Step 2 Create the GitHub integration</Text>
              <Box width="600px">
                <FieldInput
                  value={integrationName}
                  label="Give this GitHub integration a name"
                  placeholder="Integration Name"
                  onChange={e => handleIntegrationNameChange(e.target.value)}
                />
              </Box>
            </Container>
            {attempt.status === 'failed' && (
              <Flex>
                <Icons.Warning mr={2} color="error.main" size="small" />
                <Text color="error.main">Error: {attempt.statusText}</Text>
              </Flex>
            )}
            <Box mt={6}>
              <ButtonPrimary
                onClick={() => handleOnCreate(validator)}
                disabled={
                  attempt.status === 'processing'
                }
              >
                Create Integration
              </ButtonPrimary>
            </Box>
          </>
        )}
      </Validation>
    </Box>
  );
}

const Container = styled(Box)`
  max-width: 1000px;
  background-color: ${p => p.theme.colors.spotBackground[0]};
  border-radius: ${p => `${p.theme.space[2]}px`};
  padding: ${p => p.theme.space[3]}px;
`;
