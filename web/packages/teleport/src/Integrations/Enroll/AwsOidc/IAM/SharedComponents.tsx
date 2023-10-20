/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled, { useTheme } from 'styled-components';

import { AWSIcon, ChevronRightIcon } from 'design/SVGIcon';

import useTeleport from 'teleport/useTeleport';

import { BreadcrumbIconContainer } from './common';

const Container = styled.div`
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 630px;
  overflow: hidden;
`;

const Header = styled.div`
  background: #232e3e;
  height: 32px;
  padding: 0 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
`;

const HeaderLogo = styled.div`
  height: 25px;
`;

const HeaderUsername = styled.div`
  color: white;
  font-size: 12px;
`;

export function BreadcrumbArrow() {
  return (
    <BreadcrumbIconContainer>
      <ChevronRightIcon fill={'rgba(0, 0, 0, 0.8)'} size={10} />
    </BreadcrumbIconContainer>
  );
}

export function AWSWrapper(props: React.PropsWithChildren<unknown>) {
  const ctx = useTeleport();
  const theme = useTheme();

  return (
    <Container>
      <Header>
        <HeaderLogo>
          <AWSIcon size={24} fill={theme.colors.light} />
        </HeaderLogo>

        <HeaderUsername>{ctx.storeUser.state.username}</HeaderUsername>
      </Header>

      {props.children}
    </Container>
  );
}
