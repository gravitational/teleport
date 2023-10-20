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
import { storiesOf } from '@storybook/react';
import { Box } from 'design';

import SplitPane from './SplitPane';

storiesOf('Shared', module).add('SplitPane', () => {
  return (
    <Container>
      <SplitPane defaultSize="50%" flex="1" split="vertical">
        <Box flex="1" bg="red">
          red
        </Box>
        <SplitPane flex="1" split="horizontal" defaultSize="50%">
          <Box flex="1" bg="blue">
            blue
          </Box>
          <Box flex="1" bg="green">
            green
          </Box>
        </SplitPane>
      </SplitPane>
    </Container>
  );
});

const Container = styled.div`
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  position: absolute;
  display: flex;
`;
