import React from 'react';
import { Flex } from 'design';
import QuickInput from '../QuickInput';
import { Connections } from './Connections';
import { Clusters } from './Clusters';
import { Identity } from './Identity';
import styled from 'styled-components';

export function TopBar() {
  return (
    <Flex
      justifyContent="space-between"
      p="8px 25px"
      height="56px"
      alignItems="center"
    >
      <Connections />
      <CentralContainer>
        <Clusters />
        <QuickInput />
      </CentralContainer>
      <Identity />
    </Flex>
  );
}

const CentralContainer = styled.div`
  display: grid;
  column-gap: 10px;
  margin: auto 10px;
  grid-template-columns: minmax(150px, 280px) auto;
  align-items: center;
  height: 100%;
`
