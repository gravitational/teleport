/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';

import {
  RemoteCommandIcon,
  SearchIcon,
  ServerIcon,
  UpgradeIcon,
} from 'design/SVGIcon';

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
        Analyze audit logs
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
