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
