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
import { rotate360 } from 'design';

import { MonospacedOutput } from 'teleport/Assist/shared/MonospacedOutput';

interface CommandResultEntryProps {
  nodeId: string;
  nodeName: string;
  output: string;
  finished: boolean;
  errorMessage?: string;
}

const Container = styled.div`
  border-radius: 10px;
  font-size: 18px;
  position: relative;
`;

const Title = styled.div`
  font-size: 15px;
  font-weight: 600;
  padding: 10px 15px;
`;

const Header = styled.div`
  display: flex;
  justify-content: space-between;
  padding-right: 20px;
`;

const Spinner = styled.div`
  width: 20px;
  height: 20px;

  &:after {
    content: ' ';
    display: block;
    width: 12px;
    height: 12px;
    margin: 8px;
    border-radius: 50%;
    border: 3px solid ${p => p.theme.colors.text.main};
    border-color: ${p => p.theme.colors.text.main} transparent
      ${p => p.theme.colors.text.main} transparent;
    animation: ${rotate360} 1.2s linear infinite;
  }
`;

const SpinnerContainer = styled.div`
  position: relative;
  top: 4px;
`;

export function CommandResultEntry(props: CommandResultEntryProps) {
  return (
    <Container>
      <Header>
        <Title>Command output for {props.nodeName || props.nodeId}</Title>
        {!props.finished && (
          <SpinnerContainer>
            <Spinner />
          </SpinnerContainer>
        )}
      </Header>

      <MonospacedOutput>
        {props.errorMessage ? props.errorMessage : props.output}
      </MonospacedOutput>
    </Container>
  );
}
