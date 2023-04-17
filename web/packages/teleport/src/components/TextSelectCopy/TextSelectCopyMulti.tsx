/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useRef } from 'react';
import styled from 'styled-components';
import copyToClipboard from 'design/utils/copyToClipboard';
import selectElementContent from 'design/utils/selectElementContent';
import { ButtonSecondary, Box, Flex } from 'design';
import { Copy, Check } from 'design/Icon';

const ONE_SECOND_IN_MS = 1000;

export function TextSelectCopyMulti({ lines, bash = true }: Props) {
  const refs = useRef<HTMLElement[]>([]);

  function onCopyClick(index) {
    copyToClipboard(lines[index].text).then(() => {
      const targetEl =
        refs.current[index].getElementsByClassName('icon-container')[0];
      targetEl.classList.toggle('copied');

      setTimeout(() => {
        targetEl.classList.toggle('copied');
      }, ONE_SECOND_IN_MS);
    });

    const targetEl =
      refs.current[index].getElementsByClassName('text-to-copy')[0];
    selectElementContent(targetEl as HTMLElement);
  }

  const isFirefox = window.navigator?.userAgent
    ?.toLowerCase()
    .includes('firefox');

  return (
    <Box
      bg="bgTerminal"
      pl={3}
      pt={2}
      pr={7}
      borderRadius={2}
      // Firefox does not add space for visible scrollbars
      // like it does for chrome and safari.
      pb={isFirefox ? 3 : 2}
      css={{
        position: 'relative',
      }}
    >
      <Lines mr={1}>
        {lines.map((line, index) => {
          const isLastText = index === lines.length - 1;
          return (
            <Box
              pt={2}
              pb={isLastText ? 0 : 2}
              key={index}
              ref={s => (refs.current[index] = s)}
            >
              {line.comment && <Comment>{line.comment}</Comment>}
              <Flex>
                <Flex>
                  {bash && <Box mr="1">{`$`}</Box>}
                  <div className="text-to-copy">
                    <pre css={{ margin: 0 }}>{line.text}</pre>
                  </div>
                </Flex>
                <Box
                  pr={3}
                  css={`
                    position: absolute;
                    right: 0px;
                  `}
                >
                  <ButtonCopyCheck onClick={() => onCopyClick(index)}>
                    <Icon className="icon-container" color="dark">
                      <Copy data-testid="btn-copy" color="light" />
                      <Check data-testid="btn-check" />
                    </Icon>
                  </ButtonCopyCheck>
                </Box>
              </Flex>
            </Box>
          );
        })}
      </Lines>
    </Box>
  );
}

const Icon = styled.div`
  .icon-check {
    display: none;
  }
  .icon-copy {
    display: block;
  }

  &.copied {
    .icon-check {
      display: block;
    }
    .icon-copy {
      display: none;
    }
  }
`;

const Comment = styled.div`
  color: rgb(117 113 94 / 80%);
`;

const ButtonCopyCheck = styled(ButtonSecondary)`
  height: 28px;
  width: 28px;
  border-radius: 20px;
  min-height: auto;
  padding: 0;
  margin-top: -4px;
  background: rgba(255, 255, 255, 0.07);
  &:hover,
  &:focus {
    background: rgba(255, 255, 255, 0.13);
  }
`;

const Lines = styled(Box)`
  white-space: pre;
  word-break: break-all;
  font-size: 12px;
  font-family: ${({ theme }) => theme.fonts.mono};
  overflow: scroll;
  line-height: 20px;
  color: ${props => props.theme.colors.light};
`;

type Line = {
  // text is the text to copy.
  text: string;
  // comment is an optional grayed out text that
  // will render above the text to copy.
  comment?: string;
};

export type Props = {
  lines: Line[];
  // bash is a flag that when true will append a
  // `$` sign in front of the lines text.
  bash?: boolean;
};
