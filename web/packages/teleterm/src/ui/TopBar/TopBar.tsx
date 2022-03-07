import React from 'react';
import { Flex } from 'design';
import QuickInput from '../QuickInput';
import { Connections } from './Connections';
import { Identity } from './Identity';

export function TopBar() {
  return (
    <Flex
      justifyContent="space-between"
      p="0 25px"
      height="50px"
      alignItems="center"
    >
      <Connections />
      <Flex m="0 10px" justifyContent="space-between" alignItems="center">
        <QuickInput />
      </Flex>
      <Identity />
    </Flex>
  );
}
