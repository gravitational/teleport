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

import styled from 'styled-components';

import { Box, Flex, ResourceIcon } from 'design';
import { AuthProviderType } from 'shared/services';

export default function getSsoIcon(kind: AuthProviderType) {
  const desc = formatConnectorTypeDesc(kind);

  switch (kind) {
    case 'github':
      return {
        SsoIcon: () => (
          <Flex height="88px" alignItems="center">
            <ResourceIcon name="github" width="48px" />
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
              <StyledResourceIcon name="onelogin" />
            </SmIcon>
            <SmIcon>
              <StyledResourceIcon name="okta" />
            </SmIcon>
            <SmIcon mt="1">
              <StyledResourceIcon name="auth0" />
            </SmIcon>
            <SmIcon mt="1">
              <StyledResourceIcon name="entraid" />
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
              <StyledResourceIcon name="aws" />
            </SmIcon>
            <SmIcon>
              <StyledResourceIcon name="google" />
            </SmIcon>
            <SmIcon mt="1">
              <StyledResourceIcon name="gitlab" />
            </SmIcon>
            <SmIcon mt="1">
              <StyledResourceIcon name="microsoft" />
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
  width: 83px;
  flex-wrap: wrap;
  gap: 3px;
  padding: 7px;
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 8px;
`;

const SmIcon = styled(Box)`
  width: ${p => p.theme.space[5]}px;
  height: ${p => p.theme.space[5]}px;
  line-height: ${p => p.theme.space[5]}px;
  background: ${p => p.theme.colors.levels.popout};
  border-radius: 50%;
  display: flex;
  justify-content: center;
`;

const StyledResourceIcon = styled(ResourceIcon).attrs({
  width: '20px',
})``;
