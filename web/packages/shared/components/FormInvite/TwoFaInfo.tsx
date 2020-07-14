/*
Copyright 2019 Gravitational, Inc.

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
import styled from 'styled-components';
import { Box, Text, ButtonLink } from 'design';
import { Auth2faType } from 'shared/services';

const U2F_HELP_URL = 'https://support.google.com/accounts/answer/6103523?hl=en';

export default function TwoFAData({ auth2faType, qr }: Props) {
  const imgSrc = `data:image/png;base64,${qr}`;

  if (auth2faType === 'otp') {
    return (
      <div>
        <Text typography="paragraph2" mb={3}>
          Scan the bar code with Google Authenticator to generate a two factor
          token.
        </Text>
        <img width="152" src={imgSrc} style={{ border: '8px solid' }} />
        <ButtonLink
          width="100%"
          kind="secondary"
          target="_blank"
          size="small"
          href="https://support.google.com/accounts/answer/1066447?co=GENIE.Platform%3DiOS&hl=en&oco=0"
          rel="noreferrer"
        >
          Download Google Authenticator
        </ButtonLink>
      </div>
    );
  }

  if (auth2faType === 'u2f') {
    return (
      <div>
        <Text typography="h5" mb="2">
          Insert your U2F key
        </Text>
        <Box color="text.primary">
          <Text typography="paragraph2" mb={3}>
            Press the button on the U2F key after you press the create account
            button
          </Text>
          <Text typography="paragraph2" mb={3}>
            <StyledLink target="_blank" href={U2F_HELP_URL}>
              Learn more
            </StyledLink>{' '}
            about U2F 2-Step Verification.
          </Text>
        </Box>
      </div>
    );
  }

  return null;
}

const StyledLink = styled.a`
  color: ${props => props.theme.colors.light};
`;

type Props = {
  qr: string;
  auth2faType: Auth2faType;
};
