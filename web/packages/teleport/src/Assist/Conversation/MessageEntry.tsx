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

import React from 'react';
import styled from 'styled-components';

import { markdownCSS } from 'teleport/Assist/markdown';
import { Markdown } from 'teleport/Assist/Conversation/Markdown';

interface MessageEntryProps {
  content: string;
  markdown: boolean;
}

const Container = styled.div`
  padding: 10px 15px 0 17px;
  word-break: break-word;

  ${markdownCSS}
`;

export function MessageEntry(props: MessageEntryProps) {
  if (!props.markdown) {
    return (
      <Container>
        <p>{props.content}</p>
      </Container>
    );
  }

  return (
    <Container>
      <Markdown content={props.content} />
    </Container>
  );
}
