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

import React, { forwardRef } from 'react';
import styled from 'styled-components';
import { darken, lighten } from 'design/theme/utils/colorManipulator';
import * as Icons from 'design/Icon';

import { Button } from 'design/Button';

import { AuthProviderType } from 'shared/services';

const ButtonSso = forwardRef<HTMLInputElement, Props>((props: Props, ref) => {
  const { ssoType = 'unknown', title, ...rest } = props;
  const { color, Icon } = getSSOIcon(ssoType);

  return (
    <StyledButton color={color} block {...rest} ref={ref}>
      {Boolean(Icon) && (
        <IconBox>
          <Icon data-testid="icon" color="white" />
        </IconBox>
      )}
      {title}
    </StyledButton>
  );
});

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
      return { color: '#444444', Icon: Icons.GitHub, type };
    case 'bitbucket':
      return { color: '#205081', Icon: Icons.Key, /*temporary icon */ type };
    case 'google':
      return { color: '#dd4b39', Icon: Icons.Google, type };
    default:
      // provide default icon for unknown social providers
      return { color: '#f7931e', Icon: Icons.Key /*temporary icon */ };
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

  svg {
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
