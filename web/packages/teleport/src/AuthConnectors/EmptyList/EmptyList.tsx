/*
Copyright 2020-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { Card, Flex, Text } from 'design';
import * as Icons from 'design/Icon';
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
          <Icons.Github
            style={{ textAlign: 'center' }}
            fontSize="72px"
            css={`
              color: ${props => props.theme.colors.text.main};
            `}
            mr={5}
          />
        </Flex>
      </Flex>

      <Text typography="body2" mt={4} fontSize="18px" color="text.primary" bold>
        GitHub
      </Text>
      {
        <Text mt={2} color="text.slightlyMuted" transform="none">
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
        <Text mt={2} color="text.primary" transform="none">
          {info}
        </Text>
      )}
    </ConnectorBox>
  );
}

type Props = {
  onCreate: ResourceState['create'];
};
