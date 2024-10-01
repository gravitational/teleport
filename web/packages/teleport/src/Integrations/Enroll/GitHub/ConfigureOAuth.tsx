/**
 *
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

import { Box, ButtonPrimary, ButtonSecondary, Flex, Link, Text } from 'design';
import Validation, { Validator } from 'shared/components/Validation';
import React, { useState } from 'react';
import styled from 'styled-components';
import FieldInput from 'shared/components/FieldInput';
import { TextSelectCopy, TextSelectCopyMulti } from 'shared/components/TextSelectCopy';

import cfg from 'teleport/config';
import { Header } from 'teleport/Discover/Shared';

export function ConfigureOAuth({
  cluster,
  onCreatedSpec,
  }){

  const [orgName, setOrgName] = useState('');
  const [clientID, setClientID] = useState('');
  const [clientSecret, setClientSecret] = useState('');
  const homePageURL = "https://" + cluster.publicURL;
  const callbackURL = "https://" + cluster.publicURL + "/v1/webapi/github/callback"

  function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }
    onCreatedSpec({
      organization: orgName,
      connectorClientID: clientID,
      connectorClientSecret: clientSecret,
      connectorRedirectURL: callbackURL,
    });
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
              <Text bold>Step 1 Configure an OAuth App</Text>
              <Text>
                The first setup step is to configure a GitHub OAuth app, which
                Teleport will use to retrieve login information from your users'
                browsers.
              </Text>

              <br />
              <Box width="600px">
                <FieldInput
                  autoFocus={true}
                  value={orgName}
                  placeholder="my-github-organization"
                  label="Enter your GitHub organization name:"
                  onChange={e => setOrgName(e.target.value)}
                />
              </Box>

              <Text>
                Now go to your organization's{' '}
                <Link
                  target="_blank"
                  href={`https://github.com/organizations/${orgName}/settings/applications`}
                >
                  OAuth apps page
                </Link>{' '}
                under developer settings and click on "New OAuth app". Fill in
                the app with the following information:
              </Text>
              <TextSelectCopyMulti
                lines={[
                  {
                    comment: 'Application Name:',
                    text: cfg.proxyCluster,
                  },
                  {
                    comment: 'Homepage URL:',
                    text: homePageURL,
                  },
                  {
                    comment: 'Authorization callback URL:',
                    text: callbackURL,
                  },
                ]}
                bash={false}
              />

              <br />
              <Text>
                Click on "Register Application" and then "Generate a new client
                secret". Copy the ClientID and Client Secret below:
              </Text>
              <Box width="600px">
                <FieldInput
                  value={clientID}
                  placeholder="my-client-id"
                  label="ClientID:"
                  onChange={e => setClientID(e.target.value)}
                />
                <FieldInput
                  type={"password"}
                  value={clientSecret}
                  placeholder="my-client-secret"
                  label="Client Secret:"
                  onChange={e => setClientSecret(e.target.value)}
                />
              </Box>
              <Box mt={6}>
                <ButtonPrimary
                  onClick={() => handleOnCreate(validator)}
                  disabled={!orgName || !clientID || !clientSecret}
                >
                  Next
                </ButtonPrimary>
              </Box>
            </Container>
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
