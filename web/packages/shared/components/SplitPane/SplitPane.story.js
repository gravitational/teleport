/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
