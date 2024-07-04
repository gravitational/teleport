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
import styled from 'styled-components';
import { AuthProviderType } from 'shared/services';
import { GitHubIcon } from 'design/SVGIcon';
import { Box, Flex, Image } from 'design';

import iconAuth0 from './assets/saml-auth0.svg';
import iconAzureAD from './assets/saml-azuread.svg';
import iconOkta from './assets/saml-okta.svg';
import iconOneLogin from './assets/saml-one.svg';
import iconAmazon from './assets/oidc-amazon.svg';
import iconGitlab from './assets/oidc-gitlab.svg';
import iconGoogle from './assets/oidc-google.svg';
import iconWindows from './assets/oidc-windows.svg';

export default function getSsoIcon(kind: AuthProviderType) {
  const desc = formatConnectorTypeDesc(kind);

  switch (kind) {
    case 'github':
      return {
        SsoIcon: props => (
          <Flex height="72px" alignItems="center">
            <GitHubIcon
              style={{ textAlign: 'center' }}
              size={48}
              color="text.main"
              {...props}
            />
          </Flex>
        ),
        desc,
        info: 'Sign in using your GitHub account',
      };
    case 'saml':
      return {
        SsoIcon: () => (
          <MultiIconContainer>
            <SmIcon>
              <Image src={iconOneLogin} />
            </SmIcon>
            <SmIcon>
              <Image src={iconOkta} />
            </SmIcon>
            <SmIcon mt="1">
              <Image src={iconAuth0} />
            </SmIcon>
            <SmIcon mt="1">
              <Image src={iconAzureAD} />
            </SmIcon>
          </MultiIconContainer>
        ),
        desc,
        info: 'Okta, OneLogin, Microsoft Entra ID, etc.',
      };
    case 'oidc':
    default:
      return {
        SsoIcon: () => (
          <MultiIconContainer>
            <SmIcon>
              <Image src={iconAmazon} />
            </SmIcon>
            <SmIcon>
              <Image src={iconGoogle} />
            </SmIcon>
            <SmIcon mt="1">
              <Image src={iconGitlab} />
            </SmIcon>
            <SmIcon mt="1">
              <Image src={iconWindows} />
            </SmIcon>
          </MultiIconContainer>
        ),
        desc,
        info: 'Google, GitLab, Amazon and more',
      };
  }
}

function formatConnectorTypeDesc(kind) {
  kind = kind || '';
  if (kind == 'github') {
    return `GitHub`;
  }
  return kind.toUpperCase();
}

const MultiIconContainer = styled(Flex)`
  width: 67px;
  flex-wrap: wrap;
  gap: 3px;
  padding: 7px;
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 8px;
`;

const SmIcon = styled(Box)`
  width: ${p => p.theme.space[4]}px;
  height: ${p => p.theme.space[4]}px;
  line-height: ${p => p.theme.space[4]}px;
  background: white;
  color: black;
  border-radius: 50%;
`;
