/**
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

import { useEffect, useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonPrimary, Flex, Indicator } from 'design';
import { fade } from 'design/theme/utils/colorManipulator';

interface CliCommandProps {
  cliCommand: string;
  onButtonClick(): void;
  isLoading: boolean;
  buttonText?: string;
}

export function CliCommand({
  cliCommand,
  onButtonClick,
  isLoading,
  buttonText = 'Run',
}: CliCommandProps) {
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
      <CommandContainer
        mr="2"
        width="100%"
        shouldDisplayIsLoading={shouldDisplayIsLoading}
      >
        <Box mr="1">{`$`}</Box>
        <span>{cliCommand}</span>
        {shouldDisplayIsLoading && (
          <Indicator
            size={24}
            delay="none"
            css={`
              line-height: 0;
              display: inline;
              margin: auto 0 auto auto;
            `}
          />
        )}
      </CommandContainer>
      <ButtonPrimary
        onClick={onButtonClick}
        disabled={shouldDisplayIsLoading}
        css={`
          max-width: 48px;
          width: 100%;
          padding: 4px 8px;
          min-height: 10px;
          font-size: 10px;
        `}
      >
        {buttonText}
      </ButtonPrimary>
    </Flex>
  );
}

const CommandContainer = styled(Flex)<{ shouldDisplayIsLoading?: boolean }>`
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
`;
