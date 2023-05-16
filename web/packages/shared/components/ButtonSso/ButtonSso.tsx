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
import Button from 'design/Button';
import { darken, lighten } from 'design/theme/utils/colorManipulator';
import * as Icons from 'design/Icon';

import { AuthProviderType } from 'shared/services';

const ButtonSso = (props: Props) => {
  const { ssoType = 'unknown', title, ...rest } = props;
  const { color, Icon } = getSSOIcon(ssoType);

  return (
    <StyledButton color={color} block {...rest}>
      {Boolean(Icon) && (
        <IconBox>
          <Icon data-testid="icon" color="white" />
        </IconBox>
      )}
      {title}
    </StyledButton>
  );
};

type Props = {
  ssoType: SSOType;
  title: string;
  // TS: temporary handles ...styles
  [key: string]: any;
};

type SSOType =
  | 'microsoft'
  | 'github'
  | 'bitbucket'
  | 'google'
  | 'openid'
  | 'unknown';

function getSSOIcon(type: SSOType) {
  switch (type.toLowerCase()) {
    case 'microsoft':
      return { color: '#2672ec', Icon: Icons.Windows, type };
    case 'github':
      return { color: '#444444', Icon: Icons.Github, type };
    case 'bitbucket':
      return { color: '#205081', Icon: Icons.BitBucket, type };
    case 'google':
      return { color: '#dd4b39', Icon: Icons.Google, type };
    default:
      // provide default icon for unknown social providers
      return { color: '#f7931e', Icon: Icons.OpenID };
  }
}

export function guessProviderType(
  displayName = '',
  providerType: AuthProviderType
): SSOType {
  const name = displayName.toLowerCase();

  if (name.indexOf('microsoft') !== -1) {
    return 'microsoft';
  }

  if (name.indexOf('bitbucket') !== -1) {
    return 'bitbucket';
  }

  if (name.indexOf('google') !== -1) {
    return 'google';
  }

  if (name.indexOf('github') !== -1 || providerType === 'github') {
    return 'github';
  }

  if (providerType === 'oidc') {
    return 'openid';
  }

  return 'unknown';
}

const StyledButton = styled(Button)`
  background-color: ${props => props.color};
  display: block;
  width: 100%;
  border: 1px solid transparent;
  color: white;

  &:hover,
  &:focus {
    background: ${props => darken(props.color, 0.1)};
    border: 1px solid ${props => lighten(props.color, 0.4)};
  }
  height: 40px;
  position: relative;
  box-sizing: border-box;

  ${Icons.default} {
    font-size: 20px;
    opacity: 0.87;
  }
`;

const IconBox = styled.div`
  align-items: center;
  display: flex;
  justify-content: center;
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 56px;
  font-size: 24px;
  text-align: center;
  border-right: 1px solid rgba(0, 0, 0, 0.12);
`;

export default ButtonSso;
