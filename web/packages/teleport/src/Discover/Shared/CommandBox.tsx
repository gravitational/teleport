import React from 'react';
import styled from 'styled-components';

import { Box, Text } from 'design';

const Container = styled(Box)`
  max-width: 1000px;
  background-color: rgba(255, 255, 255, 0.05);
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
  border: 2px solid #2f3659;
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
