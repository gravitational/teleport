import React from 'react';

import styled from 'styled-components';

import QuickInput from '../QuickInput';

import { Connections } from './Connections';
import { Clusters } from './Clusters';
import { Identity } from './Identity';
import { NavigationMenu } from './NavigationMenu';

export function TopBar() {
  return (
    <Grid>
      <JustifyLeft>
        <Connections />
      </JustifyLeft>
      <CentralContainer>
        <Clusters />
        <QuickInput />
      </CentralContainer>
      <JustifyRight>
        <NavigationMenu />
        <Identity />
      </JustifyRight>
    </Grid>
  );
}

const Grid = styled.div`
  background: ${props => props.theme.colors.primary.main};
  display: grid;
  grid-template-columns: 1fr minmax(0, 700px) 1fr;
  width: 100%;
  padding: 8px 16px;
  height: 56px;
  box-sizing: border-box;
  align-items: center;
`;

const CentralContainer = styled.div`
  display: grid;
  column-gap: 12px;
  margin: auto 12px;
  grid-auto-flow: column;
  grid-auto-columns: 2fr 5fr; // 1fr for a single child, 2fr 5fr for two children
  align-items: center;
  height: 100%;
`;

const JustifyLeft = styled.div`
  display: flex;
  justify-self: start;
  align-items: center;
  height: 100%;
`;

const JustifyRight = styled.div`
  display: flex;
  justify-self: end;
  align-items: center;
  height: 100%;
`;
