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

export default function getSsoIcon(
  kind: AuthProviderType,
  name?: string
): () => JSX.Element {
  const guessedIcon = guessIconFromName(name || '');
  if (guessedIcon) {
    return guessedIcon;
  }

  switch (kind) {
    case 'github':
      return () => (
        <Flex height="61px" alignItems="center" justifyContent="center">
          <ResourceIcon name="github" width="61px" />
        </Flex>
      );
    case 'saml':
      return () => (
        <MultiIconContainer>
          <SmIcon>
            <StyledResourceIcon name="onelogin" />
          </SmIcon>
          <SmIcon>
            <StyledResourceIcon name="oktaAlt" />
          </SmIcon>
          <SmIcon>
            <StyledResourceIcon name="auth0" />
          </SmIcon>
          <SmIcon>
            <StyledResourceIcon name="entraid" />
          </SmIcon>
        </MultiIconContainer>
      );
    case 'oidc':
    default:
      return () => (
        <MultiIconContainer>
          <SmIcon>
            <StyledResourceIcon name="aws" />
          </SmIcon>
          <SmIcon>
            <StyledResourceIcon name="google" />
          </SmIcon>
          <SmIcon>
            <StyledResourceIcon name="gitlab" />
          </SmIcon>
          <SmIcon>
            <StyledResourceIcon name="microsoft" />
          </SmIcon>
        </MultiIconContainer>
      );
  }
}

function guessIconFromName(connectorName: string) {
  const name = connectorName.toLocaleLowerCase();

  if (name.includes('okta')) {
    return () => (
      <Flex height="61px" alignItems="center" justifyContent="center">
        <ResourceIcon name="okta" width="61px" />
      </Flex>
    );
  }
  if (
    name.includes('entra') ||
    name.includes('active directory') ||
    name.includes('microsoft') ||
    name.includes('azure')
  ) {
    return () => (
      <Flex height="61px" alignItems="center" justifyContent="center">
        <ResourceIcon name="entraid" width="61px" />
      </Flex>
    );
  }
  if (name.includes('google')) {
    return () => (
      <Flex height="61px" alignItems="center" justifyContent="center">
        <ResourceIcon name="google" width="61px" />
      </Flex>
    );
  }
  if (name.includes('gitlab')) {
    return () => (
      <Flex height="61px" alignItems="center" justifyContent="center">
        <ResourceIcon name="gitlab" width="61px" />
      </Flex>
    );
  }
  if (name.includes('onelogin')) {
    return () => (
      <Flex height="61px" alignItems="center" justifyContent="center">
        <ResourceIcon name="onelogin" width="61px" />
      </Flex>
    );
  }
  if (name.includes('auth0') || name.includes('authzero')) {
    return () => (
      <Flex height="61px" alignItems="center" justifyContent="center">
        <ResourceIcon name="auth0" width="61px" />
      </Flex>
    );
  }
}

const MultiIconContainer = styled(Flex)`
  width: 61px;
  height: 61px;
  flex-wrap: wrap;
  gap: 3px;
  padding: ${p => p.theme.space[2]}px;
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[2]};
  border-radius: 8px;
`;

const SmIcon = styled(Box)`
  width: 20px;
  height: 20px;
  display: flex;
  justify-content: center;
  align-items: center;
`;

const StyledResourceIcon = styled(ResourceIcon).attrs({
  width: '20px',
})`
  line-height: 0px !important;
`;
