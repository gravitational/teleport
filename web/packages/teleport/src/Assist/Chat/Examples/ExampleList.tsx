import React from 'react';
import styled from 'styled-components';

import { ServerIcon } from '../../Icons/ServerIcon';
import { SearchIcon } from '../../Icons/SearchIcon';
import { RemoteCommandIcon } from '../../Icons/RemoteCommandIcon';
import { UpgradeIcon } from '../../Icons/UpgradeIcon';

import { ExampleItem } from './ExampleItem';

const Container = styled.div`
  display: flex;
  flex-wrap: wrap;
  margin-top: 10px;
  margin-bottom: 30px;

  > * {
    margin-top: 10px;
  }
`;

export function ExampleList() {
  return (
    <Container>
      <ExampleItem style={{ animationDelay: '1s' }}>
        <ServerIcon size={24} />
        Connect to a server
      </ExampleItem>
      <ExampleItem style={{ animationDelay: '2s' }}>
        <SearchIcon />
        Analyse audit logs
      </ExampleItem>
      <ExampleItem style={{ animationDelay: '3s' }}>
        <RemoteCommandIcon /> Run remote commands
      </ExampleItem>
      <ExampleItem style={{ animationDelay: '4s' }}>
        <UpgradeIcon size={24} />
        Upgrade your nodes
      </ExampleItem>
    </Container>
  );
}
