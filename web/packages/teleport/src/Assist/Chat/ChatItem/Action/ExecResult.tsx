/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useState } from 'react';
import styled from 'styled-components';

import { ExecOutput } from '../../../services/messages';

interface ExecResultProps {
  result: ExecOutput;
}

const Container = styled.div`
  margin-top: 20px;
  margin-bottom: 40px;
  background: rgba(0, 0, 0, 0.2);
  padding: 15px 20px;
  border-radius: 10px;
  font-size: 18px;
  position: relative;
`;

const Title = styled.div`
  font-size: 15px;
  margin-bottom: 15px;
`;

const ActionButton = styled.div`
  font-size: 14px;
  background: rgba(0, 0, 0, 0.7);
  padding: 5px 10px;
  position: absolute;
  top: 10px;
  right: 15px;
  border-radius: 5px;
  cursor: pointer;
  &:hover {
    text-decoration: underline;
  }
`;

const CommandOutput = styled.div`
  margin-bottom: 15px;

  &:last-of-type {
    margin-bottom: 0;
  }
`;

const MachineName = styled.div`
  font-size: 15px;
  margin-bottom: 5px;
`;

const Output = styled.div`
  background: rgba(255, 255, 255, 0.1);
  border-radius: 5px;
  padding: 5px 10px;
  font-family: SFMono-Regular, Consolas, Liberation Mono, Menlo, Courier,
    monospace;
  font-size: 16px;
`;

export function ExecResult(props: ExecResultProps) {
  const [showOutput, setShowOutput] = useState(false);

  if (showOutput) {
    const items = [];

    for (const [index, item] of props.result.commandOutputs.entries()) {
      items.push(
        <CommandOutput key={index}>
          <MachineName>{item.serverName}</MachineName>
          <Output>{item.commandOutput}</Output>
        </CommandOutput>
      );
    }

    return (
      <Container>
        <Title>Output</Title>

        <ActionButton onClick={() => setShowOutput(false)}>
          Show result
        </ActionButton>

        {items}
      </Container>
    );
  }

  return (
    <Container>
      <Title>Result</Title>

      <ActionButton onClick={() => setShowOutput(true)}>
        Show output
      </ActionButton>

      {props.result.humanInterpretation}
    </Container>
  );
}
