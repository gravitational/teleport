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

import React, { useEffect, useState } from 'react';
import { Box, ButtonPrimary, Flex, Indicator } from 'design';
import { fade } from 'design/theme/utils/colorManipulator';

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
      bg="bgTerminal"
      mb={2}
    >
      <Flex
        mr="2"
        width="100%"
        shouldDisplayIsLoading={shouldDisplayIsLoading}
        css={`
          overflow: auto;
          white-space: pre;
          word-break: break-all;
          font-size: 12px;
          color: ${props => {
            // always use light colors
            const { light } = props.theme.colors;
            // 0.72 - text.slightlyMuted opacity
            return props.shouldDisplayIsLoading ? fade(light, 0.72) : light;
          }};
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
