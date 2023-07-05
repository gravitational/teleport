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

import { Card, Box, Text, ButtonPrimary } from 'design';

export function Motd({ message, onClick }: Props) {
  return (
    <StyledCard bg="levels.surface" my={6} mx="auto">
      <Box p={6}>
        <StyledText typography="h5" mb={3} textAlign="left">
          {message}
        </StyledText>
        <ButtonPrimary
          width="100%"
          mt={3}
          size="large"
          onClick={onClick}
          align="center"
        >
          Acknowledge
        </ButtonPrimary>
      </Box>
    </StyledCard>
  );
}

type Props = {
  message: string;
  onClick(): void;
};

const StyledCard = styled(Card)`
  overflow-y: auto;
  max-width: 600px;
  max-height: 500px;
  opacity: 1;
`;

const StyledText = styled(Text)`
  white-space: pre-wrap;
`;
