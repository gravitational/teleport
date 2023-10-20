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
import styled from 'styled-components';

import { Link } from 'react-router-dom';

import { ButtonPrimary } from 'design';

import cfg from 'teleport/config';

interface AccessRequestCreatedProps {
  accessRequestId: string;
}

const Container = styled.div`
  font-size: 16px;
  font-weight: bold;
  display: flex;
  flex-direction: column;
  gap: ${p => p.theme.space[2]}px;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  align-items: flex-start;
`;

const Header = styled.div`
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[2]}px;
`;

export function AccessRequestCreated(props: AccessRequestCreatedProps) {
  return (
    <Container>
      <Header>Access Request Created</Header>

      <ButtonPrimary
        as={Link}
        to={cfg.getAccessRequestRoute(props.accessRequestId)}
      >
        View
      </ButtonPrimary>
    </Container>
  );
}
