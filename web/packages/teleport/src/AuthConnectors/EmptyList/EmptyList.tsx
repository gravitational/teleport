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

import React from 'react';
import { Card, Flex, Text } from 'design';
import { GitHubIcon } from 'design/SVGIcon';
import { AuthProviderType } from 'shared/services';

import { ConnectorBox } from 'teleport/AuthConnectors/styles/ConnectorBox.styles';

import {
  LockedFeatureButton,
  LockedFeatureContainer,
} from 'teleport/AuthConnectors/styles/LockedFeatureContainer.styles';

import getSsoIcon from 'teleport/AuthConnectors/ssoIcons/getSsoIcon';
import { State as ResourceState } from 'teleport/components/useResources';
import { CtaEvent } from 'teleport/services/userEvent';

export default function EmptyList({ onCreate }: Props) {
  return (
    <Card
      color="text.main"
      p={5}
      textAlign="center"
      style={{ boxShadow: 'none' }}
    >
      <Text typography="h3" textAlign="center">
        Select a service provider below
      </Text>
      <Flex flexWrap="wrap" justifyContent="center" mt={4} minWidth="224px">
        {renderGithubConnector(onCreate)}
        <LockedFeatureContainer>
          {renderLockedItem('oidc')}
          {renderLockedItem('saml')}
          <LockedFeatureButton event={CtaEvent.CTA_AUTH_CONNECTOR}>
            Unlock OIDC & SAML with Teleport Enterprise
          </LockedFeatureButton>
        </LockedFeatureContainer>
      </Flex>
    </Card>
  );
}

function renderGithubConnector(onCreate) {
  return (
    <ConnectorBox as="button" onClick={onCreate}>
      <Flex width="100%">
        <Flex height="72px" alignItems="center">
          <GitHubIcon style={{ textAlign: 'center' }} size={48} />
        </Flex>
      </Flex>

      <Text typography="body2" mt={4} fontSize="18px" color="text.primary" bold>
        GitHub
      </Text>
      {
        <Text mt={2} color="text.slightlyMuted">
          Sign in using your GitHub account
        </Text>
      }
    </ConnectorBox>
  );
}

function renderLockedItem(kind: AuthProviderType) {
  const { desc, SsoIcon, info } = getSsoIcon(kind);
  return (
    <ConnectorBox as="button" disabled={true}>
      <Flex width="100%">
        <SsoIcon
          fontSize="50px"
          style={{
            left: 0,
            fontSize: '72px',
          }}
        />
      </Flex>

      <Text typography="body2" mt={4} fontSize={4} color="text.primary" bold>
        {desc}
      </Text>
      {info && (
        <Text mt={2} color="text.primary">
          {info}
        </Text>
      )}
    </ConnectorBox>
  );
}

type Props = {
  onCreate: ResourceState['create'];
};
