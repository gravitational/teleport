import React, { useEffect, useState } from 'react';
import { Box, ButtonPrimary, Flex, Indicator } from 'design';

interface CliCommandProps {
  cliCommand: string;
  onRun(): void;
  isLoading: boolean;
}

export function CliCommand({ cliCommand, onRun, isLoading }: CliCommandProps) {
  const [shouldDisplayIsLoading, setShouldDisplayIsLoading] = useState(false);

  useEffect(() => {
    let timeout: ReturnType<typeof setTimeout>;
    if (isLoading) {
      timeout = setTimeout(() => {
        setShouldDisplayIsLoading(true);
      }, 200);
    } else {
      setShouldDisplayIsLoading(false);
    }

    return () => clearTimeout(timeout);
  }, [isLoading, setShouldDisplayIsLoading]);

  return (
    <Flex
      p="2"
      alignItems="center"
      justifyContent="space-between"
      borderRadius={2}
      bg="primary.dark"
      mb={2}
    >
      <Flex
        mr="2"
        color={shouldDisplayIsLoading ? 'text.secondary' : 'text.primary'}
        width="100%"
        css={`
          overflow: auto;
          white-space: pre;
          word-break: break-all;
          font-size: 12px;
          font-family: ${props => props.theme.fonts.mono};
        `}
      >
        <Box mr="1">{`$`}</Box>
        <span>{cliCommand}</span>
        {shouldDisplayIsLoading && (
          <Indicator
            fontSize="14px"
            delay="none"
            css={`
              display: inline;
              margin: auto 0 auto auto;
            `}
          />
        )}
      </Flex>
      <ButtonPrimary
        onClick={onRun}
        disabled={shouldDisplayIsLoading}
        css={`
          max-width: 48px;
          width: 100%;
          padding: 4px 8px;
          min-height: 10px;
          font-size: 10px;
        `}
      >
        Run
      </ButtonPrimary>
    </Flex>
  );
}
