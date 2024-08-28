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

import React, { useEffect, useState } from 'react';
import { Link as InternalRouteLink } from 'react-router-dom';
import { useLocation } from 'react-router';
import styled from 'styled-components';
import { Box, ButtonSecondary, Text, Link, Flex, ButtonPrimary } from 'design';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  IntegrationEnrollEvent,
  IntegrationEnrollEventData,
  IntegrationEnrollKind,
  userEventService,
} from 'teleport/services/userEvent';
import { Header } from 'teleport/Discover/Shared';

import {
  Integration,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import cfg from 'teleport/config';

import { TextSelectCopy } from 'shared/components/TextSelectCopy';

export function GitHub() {
  const [username, setUsername] = useState('');
  const [integrationName, setIntegrationName] = useState('');
  const [isIntegrationNameEdited, setIsIntegrationNameEdited] = useState(false);
  const [orgName, setOrgName] = useState('');
  const [createdIntegration, setCreatedIntegration] = useState<Integration>();
  const { attempt, run } = useAttempt('');
  const [isCAExported, setCAExported] = useState(false);
  const [isAccessSet, setAccessSet] = useState(false);

  const [eventData] = useState<IntegrationEnrollEventData>({
    id: crypto.randomUUID(),
    kind: IntegrationEnrollKind.GitHub,
  });

  useEffect(() => {
    if (!isIntegrationNameEdited) {
      setIntegrationName("github-" + orgName);
    }
  }, [orgName, isIntegrationNameEdited]);

  function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    run(() =>
      integrationService
        .createIntegration({
          name: integrationName,
          subKind: IntegrationKind.GitHub, //TODO
          github: {
            organization: orgName,
          },
        })
        .then(res => {
          setCreatedIntegration(res);
        })
    );
  }

  function handleIntegrationNameChange(value: string) {
    setIntegrationName(value);
    setIsIntegrationNameEdited(true);
  }

  if (isAccessSet) {
    return (
    <Box pt={3}>
      <Header>Create a GitHub Integration</Header>
      <Box width="800px" mb={4}>
      <Container mb={5}>
        <Box mb={4}>
          <Text>
            Now to use the integration with 'git', login to Teleport:
          </Text>
          <TextSelectCopy
            mt="2"
            text={ "tsh login --proxy=localhost:3000 --auth=local --user=steve teleport-test.stevexin.app" }
          />
          {'To clone a new repository, find the SSH url of the repository on '}
          <Link href={ `https://github.com/organizations/${createdIntegration.spec.organization}` } target="_blank" >
            github.com
          </Link>
          {' then'}
          <TextSelectCopy
            mt="2"
            text={`tsh git clone <git-clone-ssh-url>`}
          />
          {'To configure an existing git repository, go to the repository then'}
          <TextSelectCopy
            mt="2"
            text={`tsh git config update`}
          />
        <Text>
          Once the repository is cloned or configured, use 'git' as normal.
        </Text>
        </Box>
      </Container>
    <Flex gap="3">
        <ButtonSecondary as={Link} to={cfg.getUnifiedResourcesRoute("teleport-test.stevexin.app")} size="large" >
          Go To Resources List
        </ButtonSecondary>
      <ButtonPrimary as={Link} to={cfg.routes.integrations} size="large">
        Go to Integration List
      </ButtonPrimary>
    </Flex>
      </Box>
    </Box>
    )
  }
  if (isCAExported) {
    return (
    <Box pt={3}>
      <Header>Create a GitHub Integration</Header>
      <Box width="800px" mb={4}> Update your user traits to allow access to the GitHub organization "{createdIntegration.spec.organization}" and provide your GitHub username.
      </Box>
      <Container mb={5}>
        <Text bold>Step 3</Text>
    <Validation>
      {({ validator }) => (
              <Box width="600px">
          <FieldInput
            value={createdIntegration.spec.organization}
            label="GitHub organizations:"
            placeholder="GitHub organizations"
            onChange={e => setUsername(e.target.value)}
          />
          <FieldInput
            value={username}
            label="GitHub username:"
            placeholder="GitHub username"
            onChange={e => setUsername(e.target.value)}
          />
              </Box>
      )}
      </Validation>
      </Container>
      <Box mt={6}>
        <ButtonSecondary
          onClick={() => setAccessSet(true)}
        >
          Configure access
        </ButtonSecondary>
      </Box>
    </Box>
    )
  }
  if (createdIntegration) {
    return (
    <Box pt={3}>
      <Header>Create a GitHub Integration</Header>
      <Box width="800px" mb={4}>
          GitHub integration "{createdIntegration.name}" successfully added. Now we want add the SSH certificate authority to your GitHub organization.
      </Box>
      <Container mb={5}>
      <Box width="600px">
        <Text bold>Step 2</Text>
        <Text>
          Go to your organization's {' '} 
          <Link
            target="_blank"
            href={`https://github.com/organizations/${createdIntegration.spec.organization}/settings/security`}
          >
            settings page for Authentication security
          </Link> and click on "New CA":
        </Text>
        <TextSelectCopy bash={false} text={createdIntegration.spec.publicKeys[0]}>
        </TextSelectCopy>
        <br />
        <Text>
          After the CA is added, it should have the following fingerprint:
        </Text>
        <TextSelectCopy bash={false} text={createdIntegration.spec.fingerprints[0]}>
        </TextSelectCopy>
      </Box>
      </Container>
      <Box mt={6}>
        <ButtonSecondary
          onClick={() => setCAExported(true)}
        >
          Next
        </ButtonSecondary>
      </Box>
    </Box>
    )
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
              <Text bold>Step 1</Text>
              <Box width="600px">
                <FieldInput
                  autoFocus={true}
                  value={orgName}
                  placeholder="my-github-organization"
                  label="Enter your GitHub organization name"
                  onChange={e => setOrgName(e.target.value)}
                />
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
                  attempt.status === 'processing' || !orgName
                }
              >
                Create Integration
              </ButtonPrimary>
              <ButtonSecondary
                ml={3}
                as={InternalRouteLink}
                to={cfg.getIntegrationEnrollRoute(null)}
              >
                Back
              </ButtonSecondary>
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

const RouteLink = styled(InternalRouteLink)`
  color: ${({ theme }) => theme.colors.buttons.link.default};

  &:hover,
  &:focus {
    color: ${({ theme }) => theme.colors.buttons.link.hover};
  }
`;
