import { Flex, Text } from 'design';
import React from 'react';

interface StacktraceProps {
  lines: string;
}

export function StackTrace(props: StacktraceProps): JSX.Element {
  return (
    <>
      <Text mb={2}>Last 10 lines of error logs:</Text>
      <Flex
        width="100%"
        color="light"
        bg="bgTerminal"
        p={2}
        mb={2}
        flexDirection="column"
        borderRadius={1}
      >
        <span
          css={`
            white-space: pre-wrap;
            font-size: 12px;
            font-family: ${props => props.theme.fonts.mono};
          `}
        >
          {props.lines}
        </span>
      </Flex>
    </>
  );
}
