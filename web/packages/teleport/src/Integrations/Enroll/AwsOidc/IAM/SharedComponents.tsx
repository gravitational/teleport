import React from 'react';
import styled from 'styled-components';

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

  return (
    <Container>
      <Header>
        <HeaderLogo>
          <AWSIcon size={24} />
        </HeaderLogo>

        <HeaderUsername>{ctx.storeUser.state.username}</HeaderUsername>
      </Header>

      {props.children}
    </Container>
  );
}
