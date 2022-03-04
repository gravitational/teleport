/*
Copyright 2019 Gravitational, Inc.

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
import { Box, Text } from 'design';
import { ExpanderClusters } from './ExpanderClusters';
import { ExpanderConnections } from './ExpanderConnections';

export function Navigator() {
  return (
    <Nav bg="primary.main">
      <Text typography="subtitle2" m={2}>
        NAVIGATOR
      </Text>
      <Scrollable>
        <ExpanderConnections />
        <Separator />
        {/*<ExpanderClusters />*/}
        <Separator />
      </Scrollable>
    </Nav>
  );
}

const Nav = styled(Box)`
  display: flex;
  flex-direction: column;
  height: 100%;
  user-select: none;
`;

const Scrollable = styled(Box)`
  height: 100%;
  overflow: auto;
`;

const Separator = styled.div`
  background: ${props => props.theme.colors.primary.lighter};
  height: 1px;
`;
