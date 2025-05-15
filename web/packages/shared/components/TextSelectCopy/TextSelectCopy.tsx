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

import { useRef, useState } from 'react';
import { useTheme } from 'styled-components';

import { Box, ButtonPrimary, Flex } from 'design';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import selectElementContent from 'design/utils/selectElementContent';
import { wait } from 'shared/utils/wait';

export function TextSelectCopy({
  text,
  fontFamily,
  allowMultiline,
  onCopy,
  bash = true,
  ...styles
}: Props) {
  const font = fontFamily || useTheme().fonts.mono;
  const ref = useRef(undefined);
  const abortControllerRef = useRef<AbortController>(undefined);
  const [copyCmd, setCopyCmd] = useState(() => 'Copy');

  function onCopyClick() {
    abortControllerRef.current?.abort();
    abortControllerRef.current = new AbortController();
    const signal = abortControllerRef.current.signal;

    copyToClipboard(text)
      .then(() => {
        setCopyCmd('Copied');

        return wait(1_000, signal);
      })
      .then(
        () => setCopyCmd('Copy'),
        () => {} // Noop on abort.
      );

    selectElementContent(ref.current);
    onCopy && onCopy();
  }

  const boxStyles: React.CSSProperties =
    bash && !allowMultiline
      ? {
          overflow: 'auto',
          whiteSpace: 'pre',
          wordBreak: 'break-all',
          fontSize: '12px',
          fontFamily: font,
        }
      : {
          wordBreak: 'break-all',
          fontSize: '12px',
          fontFamily: font,
        };

  return (
    <Flex
      bg="bgTerminal"
      p="2"
      alignItems="center"
      justifyContent="space-between"
      borderRadius={2}
      color="light"
      {...styles}
    >
      <Flex mr="2" style={boxStyles}>
        {bash && <Box mr="1" style={{ userSelect: 'none' }}>{`$`}</Box>}
        <div ref={ref}>{text}</div>
      </Flex>
      <ButtonPrimary
        onClick={onCopyClick}
        style={{
          maxWidth: '48px',
          width: '100%',
          padding: '4px 8px',
          minHeight: '10px',
          fontSize: '10px',
        }}
      >
        {copyCmd}
      </ButtonPrimary>
    </Flex>
  );
}

type Props = {
  text: string;
  bash?: boolean;
  onCopy?: () => void;
  allowMultiline?: boolean;
  // handles styles
  [key: string]: any;
};
