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
  grid-template-columns: 1fr auto 1fr;
  width: 100%;
  padding: 8px 25px;
  height: 56px;
  box-sizing: border-box;
  align-items: center;
`;

const CentralContainer = styled.div`
  display: grid;
  column-gap: 10px;
  margin: auto 10px;
  grid-template-columns: minmax(150px, 280px) minmax(200px, 600px);
  align-items: center;
  height: 100%;
`;

const JustifyLeft = styled.div`
  display: flex;
  justify-self: start;
  align-items: center;
`;

const JustifyRight = styled.div`
  display: flex;
  justify-self: end;
  align-items: center;
`;
