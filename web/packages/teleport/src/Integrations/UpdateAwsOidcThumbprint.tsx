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

import { useState } from 'react';
import styled from 'styled-components';
import { Text, Link as ExternalLink, Flex, Box, ButtonPrimary } from 'design';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import { ToolTipInfo } from 'shared/components/ToolTip';
import useAttempt from 'shared/hooks/useAttemptNext';
import * as Icons from 'design/Icon';

import { Mark } from 'teleport/Discover/Shared';
import {
  Integration,
  integrationService,
} from 'teleport/services/integrations';
import { splitAwsIamArn } from 'teleport/services/integrations/aws';
import cfg from 'teleport/config';

export function UpdateAwsOidcThumbprint({
  integration,
}: {
  integration: Integration;
}) {
  const { attempt, run } = useAttempt();

  const [thumbprint, setThumbprint] = useState('');

  function getThumbprint() {
    run(() => integrationService.fetchThumbprint().then(setThumbprint));
  }

  const { awsAccountId, arnStartingPart } = splitAwsIamArn(
    integration.spec.roleArn
  );

  const encodedOidcProviderArn = encodeURIComponent(
    `${arnStartingPart}${awsAccountId}:oidc-provider/${cfg.proxyCluster}`
  );

  return (
    <ToolTipInfo sticky={true}>
      <Box py={1}>
        <Flex gap={2} flexDirection="column" mb={2}>
          <Text>
            This integration has no S3 bucket configured. When renewing your
            HTTPS certificate, if it has a different CA, a manual update of this
            integration's thumbprint is required.
          </Text>
          <Text>You may run into issues when the thumbprint is stale.</Text>
        </Flex>
        <Box mb={3} mt={1}>
          <ButtonPrimary
            onClick={getThumbprint}
            size="medium"
            width="100%"
            disabled={attempt.status === 'processing'}
          >
            Generate a New Thumbprint
          </ButtonPrimary>
          <Box mt={2}>
            {thumbprint && (
              <TextSelectCopyMulti
                bash={false}
                lines={[
                  {
                    text: thumbprint,
                  },
                ]}
              />
            )}
            {attempt.status === 'failed' && (
              <Flex mt={1}>
                <Icons.Warning size="small" color="error.main" mr={1} /> Error
                fetching thumbprint: some kind of error
              </Flex>
            )}
          </Box>
        </Box>

        <Ul>
          <Text bold>To update thumbprint:</Text>
          <li>
            - Go to your{' '}
            <ExternalLink
              target="_blank"
              href={`https://console.aws.amazon.com/iam/home#/identity_providers/details/OPENID/${encodedOidcProviderArn}`}
            >
              IAM Identity Provider
            </ExternalLink>{' '}
            dashboard
          </li>
          <li>
            - On <Mark>Thumbprints</Mark> section click on <Mark>Manage</Mark>{' '}
            then click on <Mark>Add Thumbprint</Mark>
          </li>
          <li>- Copy and paste the generated thumbprint</li>
        </Ul>
      </Box>
    </ToolTipInfo>
  );
}

const Ul = styled.ul`
  margin-left: 0;
  padding-left: 0;
  list-style: none;
`;
