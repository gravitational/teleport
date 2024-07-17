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

import React, { PropsWithChildren } from 'react';

import { ArrowBack } from 'design/Icon';
import { Text, ButtonIcon, Flex, H2 } from 'design';

export const Header: React.FC<PropsWithChildren<unknown>> = ({ children }) => (
  <H2 my={1}>{children}</H2>
);

export const HeaderSubtitle: React.FC<PropsWithChildren> = ({ children }) => (
  <Text mb={5}>{children}</Text>
);

export const HeaderWithBackBtn: React.FC<
  PropsWithChildren<{ onPrev(): void }>
> = ({ children, onPrev }) => (
  <Flex alignItems="center">
    <ButtonIcon size={1} title="Go Back" onClick={onPrev} ml={-2}>
      <ArrowBack size="large" />
    </ButtonIcon>
    <H2 my={1}>{children}</H2>
  </Flex>
);
