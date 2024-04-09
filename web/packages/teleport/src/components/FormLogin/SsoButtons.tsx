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
import { Box, Text } from 'design';
import ButtonSso, { guessProviderType } from 'shared/components/ButtonSso';
import { AuthProvider } from 'shared/services';

const SSOBtnList = forwardRef<HTMLInputElement, Props>(
  ({ providers, isDisabled, onClick, autoFocus = false }, ref) => {
    const style = providers.length === 1 ? { gridColumnEnd: 'span 2' } : {};
    const $btns = providers.map((item, index) => {
      let { name, type, displayName } = item;
      const title = displayName || name;
      const ssoType = guessProviderType(title, type);
      return (
        <ButtonSso
          setRef={index === 0 ? ref : null}
          key={index}
          title={title}
          ssoType={ssoType}
          disabled={isDisabled}
          autoFocus={index === 0 && autoFocus}
          style={style}
          onClick={e => {
            e.preventDefault();
            onClick(item);
          }}
        />
      );
    });

    if ($btns.length === 0) {
      return (
        <Text textAlign="center" bold pt={3}>
          You have no SSO providers configured
        </Text>
      );
    }

    return <Container data-testid="sso-list">{$btns}</Container>;
  }
);

type Props = {
  isDisabled: boolean;
  onClick(provider: AuthProvider): void;
  providers: AuthProvider[];
  // autoFocus focuses on the first button in list.
  autoFocus?: boolean;
};

const Container = styled(Box)`
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: ${p => p.theme.space[3]}px;
`;

export default SSOBtnList;
