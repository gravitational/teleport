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

import styled from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';
import { RocketLaunch } from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import { H2, Subtitle2 } from 'design/Text';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { CtaEvent } from 'teleport/services/userEvent';

export function CtaConnectors() {
  return (
    <AuthConnectorsCTABox>
      <CTALogosContainer>
        <CTALogo>
          <ResourceIcon name="google" width="36px" />
        </CTALogo>
        <CTALogo style={{ left: 70 }}>
          <ResourceIcon name="entraid" width="36px" />
        </CTALogo>
        <CTALogo style={{ left: 70 * 2 }}>
          <ResourceIcon name="gitlab" width="36px" />
        </CTALogo>
        <CTALogo style={{ left: 70 * 3 }}>
          <ResourceIcon name="microsoft" width="36px" />
        </CTALogo>
        <CTALogo style={{ left: 70 * 4 }}>
          <ResourceIcon name="okta" width="36px" />
        </CTALogo>
        <CTALogo style={{ left: 70 * 5 }}>
          <ResourceIcon name="onelogin" width="36px" />
        </CTALogo>
        <CTALogo style={{ left: 70 * 6 }}>
          <ResourceIcon name="auth0" width="36px" />
        </CTALogo>
      </CTALogosContainer>
      <Flex flexDirection="column" gap={2}>
        <H2>Unlock OIDC & SAML Single Sign-On with Teleport Enterprise</H2>
        <Subtitle2 color="text.slightlyMuted">
          Connect your identity provider to streamline employee onboarding,
          simplify compliance, and monitor access patterns against a single
          source of truth.
        </Subtitle2>
        <ButtonLockedFeature
          event={CtaEvent.CTA_AUTH_CONNECTOR}
          noIcon
          size="small"
          px={3}
          py={2}
          width="fit-content"
          mt={1}
        >
          <RocketLaunch size="small" mr={2} />
          Upgrade to Enterprise
        </ButtonLockedFeature>
      </Flex>
    </AuthConnectorsCTABox>
  );
}

const AuthConnectorsCTABox = styled(Box)`
  width: 100%;
  padding: ${p => p.theme.space[5]}px;

  border: ${props => props.theme.borders[1]};
  border-color: ${props => props.theme.colors.interactive.tonal.neutral[1]};
  border-radius: ${props => props.theme.radii[2]}px;
  background: transparent;

  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: ${p => p.theme.space[3]}px;
  align-self: stretch;
`;

const CTALogosContainer = styled(Box)`
  display: flex;
  flex-direction: row;
  width: 100%;
  // We have to manually set a height here since the CTALogo's are 'position: absolute' and won't automatically size the container.
  //padding (top and bottom) + height of the icon
  height: ${p => p.theme.space[4] * 2 + 36}px;

  position: relative;
`;

const CTALogo = styled(Box)`
  background-color: ${props => props.theme.colors.levels.sunken};
  padding: ${p => p.theme.space[4]}px;

  display: flex;
  align-items: center;
  justify-content: center;

  border: ${props => props.theme.borders[1]};
  border-color: ${props => props.theme.colors.interactive.tonal.neutral[1]};
  border-radius: 50%;

  position: absolute;
`;
