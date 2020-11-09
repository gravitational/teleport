/**
 * Copyright 2020 Gravitational, Inc.
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

import React from 'react';
import copyToClipboard from 'design/utils/copyToClipboard';
import selectElementContent from 'design/utils/selectElementContent';
import { ButtonPrimary, Text, Flex } from 'design';
import { useTheme } from 'styled-components';

export default function TextSelectCopy({ text, fontFamily, ...styles }: Props) {
  const font = fontFamily || useTheme().fonts.mono;
  const ref = React.useRef();
  const [copyCmd, setCopyCmd] = React.useState(() => 'Copy');

  function onCopyClick() {
    copyToClipboard(text).then(() => setCopyCmd('Copied'));
    selectElementContent(ref.current);
  }

  return (
    <Flex
      bg="bgTerminal"
      p="2"
      alignItems="center"
      justifyContent="space-between"
      borderRadius={2}
      {...styles}
    >
      <Text
        ref={ref}
        style={{
          wordBreak: 'break-all',
          fontSize: '12px',
          fontFamily: font,
        }}
        mr="3"
      >
        {text}
      </Text>
      <ButtonPrimary
        onClick={onCopyClick}
        style={{ padding: '4px 8px', minHeight: '10px', fontSize: '10px' }}
      >
        {copyCmd}
      </ButtonPrimary>
    </Flex>
  );
}

type Props = {
  text: string;
  // handles styles
  [key: string]: any;
};
