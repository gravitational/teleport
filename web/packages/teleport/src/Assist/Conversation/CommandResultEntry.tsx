/**
 * Copyright 2023 Gravitational, Inc.
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
import styled, { keyframes } from 'styled-components';

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

const spin = keyframes`
  to {
    transform: rotate(360deg);
  }
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
    animation: ${spin} 1.2s linear infinite;
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
