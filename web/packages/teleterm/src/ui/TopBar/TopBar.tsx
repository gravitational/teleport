import React from 'react';
import QuickInput from '../QuickInput';
import { Connections } from './Connections';
import { Clusters } from './Clusters';
import { Identity } from './Identity';
import styled from 'styled-components';

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
        <Identity />
      </JustifyRight>
    </Grid>
  );
}

const Grid = styled.div`
  display: grid;
  grid-template-columns: 1fr 4fr 2fr;
  width: 100%;
  padding: 8px 24px;
  height: 56px;
  box-sizing: border-box;
  align-items: center;
`;

const CentralContainer = styled.div`
  display: grid;
  column-gap: 12px;
  margin: auto 12px;
  grid-auto-flow: column;
  grid-auto-columns: 1fr 2fr; // 1fr for a single child, 1fr 2fr for two children
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
  display: grid;
  justify-self: end;
  align-items: center;
  height: 100%;
`;
