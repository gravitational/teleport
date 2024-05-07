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

import React, { useState } from 'react';
import styled from 'styled-components';

import { Box, Text, Flex, ButtonPrimary } from 'design';
import { StyledCheckbox } from 'design/Checkbox';

import FormLogin from 'teleport/components/FormLogin';
import { LogoHero } from 'teleport/components/LogoHero';
import cfg from 'teleport/config';

import useLogin, { State } from './useLogin';
import Motd from './Motd';

export function Login() {
  const state = useLogin();
  return <LoginComponent {...state} />;
}

export function LoginComponent({
  attempt,
  onLogin,
  onLoginWithWebauthn,
  onLoginWithSso,
  authProviders,
  auth2faType,
  checkingValidSession,
  preferredMfaType,
  isLocalAuthEnabled,
  clearAttempt,
  isPasswordlessEnabled,
  primaryAuthType,
  licenseAcknowledged,
  setLicenseAcknowledged,
  motd,
  showMotd,
  acknowledgeMotd,
}: State) {
  // while we are checking if a session is valid, we don't return anything
  // to prevent flickering. The check only happens for a frame or two so
  // we avoid rendering a loader/indicator since that will flicker as well
  if (checkingValidSession) {
    return null;
  }

  if (!licenseAcknowledged && cfg.getEdition() === 'community') {
    return (
      <LicenseAcknowledgement setLicenseAcknowledged={setLicenseAcknowledged} />
    );
  }

  return (
    <>
      <LogoHero />
      {showMotd ? (
        <Motd message={motd} onClick={acknowledgeMotd} />
      ) : (
        <FormLogin
          title={'Sign in to Teleport'}
          authProviders={authProviders}
          auth2faType={auth2faType}
          preferredMfaType={preferredMfaType}
          isLocalAuthEnabled={isLocalAuthEnabled}
          onLoginWithSso={onLoginWithSso}
          onLoginWithWebauthn={onLoginWithWebauthn}
          onLogin={onLogin}
          attempt={attempt}
          clearAttempt={clearAttempt}
          isPasswordlessEnabled={isPasswordlessEnabled}
          primaryAuthType={primaryAuthType}
        />
      )}
    </>
  );
}

const LicenseBox = styled(Box)`
  width: 100%;
  max-width: 550px;
  padding: ${props => props.theme.space[4]}px;
  background-color: ${props => props.theme.colors.levels.surface};
  border-radius: ${props => props.theme.radii[3]}px;
  margin: auto;
`;

function LicenseAcknowledgement({
  setLicenseAcknowledged,
}: {
  setLicenseAcknowledged(): void;
}) {
  const [checked, setChecked] = useState(false);

  return (
    <>
      <LogoHero />
      <LicenseBox>
        <Text
          css={`
            line-height: 32px;
            font-weight: 500;
            font-size: ${props => props.theme.fontSizes[7]}px;
          `}
        >
          Welcome to Teleport
        </Text>
        <Text
          css={`
            font-weight: 300;
            line-height: 24px;
            font-size: ${props => props.theme.fontSizes[3]}px;
            color: ${p => p.theme.colors.text.slightlyMuted};
          `}
        >
          Companies may use Teleport Community Edition on the condition they
          have less than 100 employees and less than $10MM in annual revenue. If
          your company exceeds these limits, please{' '}
          <Text
            as="a"
            href="https://goteleport.com/signup/enterprise/?utm_campaign=CTA_terms_and_conditions&utm_source=oss&utm_medium=in-product"
          >
            contact us
          </Text>{' '}
          to evaluate and use Teleport.
        </Text>
        <Flex mt={3} alignItems="center" gap={2}>
          <StyledCheckbox
            checked={checked}
            onChange={e => {
              setChecked(e.target.checked);
            }}
          />
          By clicking continue, you agree to our{' '}
          <Text
            as="a"
            href="https://github.com/gravitational/teleport/blob/master/LICENSE-community"
          >
            Terms and Conditions
          </Text>
        </Flex>
        <ButtonPrimary
          disabled={!checked}
          onClick={() => {
            setLicenseAcknowledged();
            window.location.reload();
          }}
          textTransform="none"
          block
          mt={3}
        >
          Continue
        </ButtonPrimary>
      </LicenseBox>
      <Flex
        css={`
          width: 100%;
          position: absolute;
          bottom: 24px;
          gap: 45px;
          justify-content: center;
        `}
      >
        <Text>@Gravitational, Inc. All Rights Reserved</Text>
        <Text
          as="a"
          href="https://goteleport.com/legal/tos/"
          css={`
            color: ${props => props.theme.colors.text.main};
            text-decoration: none;
          `}
        >
          Terms of Service
        </Text>
        <Text
          as="a"
          href="https://goteleport.com/legal/privacy/"
          css={`
            color: ${props => props.theme.colors.text.main};
            text-decoration: none;
          `}
        >
          Privacy Policy
        </Text>
      </Flex>
    </>
  );
}
