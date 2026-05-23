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

import { forwardRef } from 'react';
import styled from 'styled-components';

import { ButtonPrimary, ButtonProps, ButtonSecondary } from 'design/Button';
import * as Icons from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import { AuthProviderType, SSOType } from 'shared/services';

const DuoButton = styled(ButtonPrimary)`
  &,
  &:hover,
  &:focus-visible,
  &:active {
    color: #ffffff;
  }
  background-color: #1d69cc;
  &:hover,
  &:focus-visible,
  &:active {
    background-color: #0d5cbd;
  }
`;

const ButtonSso = forwardRef<HTMLButtonElement, Props>((props: Props, ref) => {
  const { ssoType = 'unknown', title, ...rest } = props;
  const ButtonVariant = ssoType === 'duo' ? DuoButton : ButtonSecondary;

  return (
    <ButtonVariant gap={3} size="extra-large" block {...rest} ref={ref}>
      <SSOIcon type={ssoType} />
      {title}
    </ButtonVariant>
  );
});

type Props = ButtonProps<'button'> & {
  ssoType: SSOType;
  title: string;
};

export function SSOIcon({ type }: { type: SSOType }) {
  const commonResourceIconProps = {
    width: '24px',
    height: '24px',
  };
  switch (type.toLowerCase()) {
    case 'microsoft':
      return <ResourceIcon name="microsoft" {...commonResourceIconProps} />;
    case 'github':
      return <ResourceIcon name="github" {...commonResourceIconProps} />;
    case 'bitbucket':
      return (
        <ResourceIcon name="atlassianbitbucket" {...commonResourceIconProps} />
      );
    case 'google':
      return <ResourceIcon name="google" {...commonResourceIconProps} />;
    case 'okta':
      return <ResourceIcon name="okta" {...commonResourceIconProps} />;
    case 'duo':
      return <ResourceIcon name="duo" width="48px" />;
    default:
      // provide default icon for unknown social providers
      return <Icons.Key data-testid="icon" />;
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

  if (name.indexOf('okta') !== -1) {
    return 'okta';
  }

  if (name.indexOf('duo') !== -1) {
    return 'duo';
  }

  if (providerType === 'oidc') {
    return 'openid';
  }

  return 'unknown';
}

export default ButtonSso;
