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
import { MonospacedOutput } from 'teleport/Assist/shared/MonospacedOutput';
import { Markdown } from 'teleport/Assist/Conversation/Markdown';

interface CommandResultSummaryEntryProps {
  command: string;
  summary: string;
}

const Container = styled.div`
  border-radius: 10px;
  position: relative;
`;

const Title = styled.div`
  font-size: 15px;
  font-weight: 600;
  padding: 10px 15px;
`;

const Summary = styled.div`
  padding: 10px 15px 0 17px;

  ${markdownCSS}
`;

const Header = styled.div`
  display: flex;
  justify-content: space-between;
  padding-right: 20px;
`;

export function CommandResultSummaryEntry(
  props: CommandResultSummaryEntryProps
) {
  return (
    <Container>
      <Header>
        <Title>Summary of command execution</Title>
      </Header>

      <MonospacedOutput>{props.command}</MonospacedOutput>

      <Summary>
        <Markdown content={props.summary} />
      </Summary>
    </Container>
  );
}
