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

import React, { useState } from 'react';
import styled from 'styled-components';

import { ButtonPrimary } from 'design';
import Flex from 'design/Flex';

import { Resources } from 'teleport/Assist/Conversation/AccessRequests/Resources';

import type { Resource } from 'teleport/Assist/types';

interface AccessRequestProps {
  resources: Resource[];
  reason: string;
}

const Container = styled.div`
  padding: 15px 15px 15px 17px;
`;

const StyledInput = styled.input<{ hasError: boolean }>`
  border: 1px solid
    ${p =>
      p.hasError
        ? p.theme.colors.error.main
        : p.theme.colors.spotBackground[0]};
  padding: 12px 15px;
  border-radius: 5px;
  font-family: ${p => p.theme.font};
  background: ${p => p.theme.colors.levels.surface};
  width: 300px;

  &:disabled {
    background: ${p => p.theme.colors.spotBackground[0]};
  }

  &:active:not(:disabled),
  &:focus:not(:disabled) {
    outline: none;
    border-color: ${p => p.theme.colors.text.slightlyMuted};
  }
`;

const InfoText = styled.span`
  display: block;
  font-size: 14px;
  font-weight: 600;
  margin: 5px 0;
`;

const SubTitle = styled.div`
  font-size: 13px;
  font-weight: 600;
  margin: 5px 0;
`;

export function AccessRequest(props: AccessRequestProps) {
  const [reason, setReason] = useState(props.reason);

  return (
    <Container>
      <InfoText style={{ marginTop: 0 }}>Create an access request</InfoText>

      <SubTitle>Resources</SubTitle>

      <Resources resources={props.resources} />

      <SubTitle>Reason</SubTitle>

      <StyledInput value={reason} onChange={e => setReason(e.target.value)} />

      <Flex mt={3} justifyContent="flex-end">
        <ButtonPrimary ml={3} onClick={() => null}>
          Create
        </ButtonPrimary>
      </Flex>
    </Container>
  );
}
