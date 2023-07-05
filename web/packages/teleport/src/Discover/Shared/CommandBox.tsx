/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';

import { Box, Text } from 'design';

const Container = styled(Box)`
  max-width: 1000px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
`;

interface CommandBoxProps {
  header?: React.ReactNode;
}

export function CommandBox(props: React.PropsWithChildren<CommandBoxProps>) {
  return (
    <Container p={3} borderRadius={3} mb={3}>
      {props.header || <Text bold>Command</Text>}
      <Box mt={3} mb={3}>
        {props.children}
      </Box>
      This script is valid for 4 hours.
    </Container>
  );
}
