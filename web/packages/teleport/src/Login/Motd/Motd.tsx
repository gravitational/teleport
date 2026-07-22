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

import styled from 'styled-components';

import { ButtonPrimary, Card, Text } from 'design';

export function Motd({ message, onClick }: Props) {
  return (
    <StyledCard bg="levels.surface" my={6} p={6} mx="auto">
      <StyledText typography="body1" mb={3} textAlign="left">
        {message}
      </StyledText>
      <ButtonPrimary width="100%" mt={3} size="large" onClick={onClick}>
        Acknowledge
      </ButtonPrimary>
    </StyledCard>
  );
}

type Props = {
  message: string;
  onClick(): void;
};

const StyledCard = styled(Card)`
  display: flex;
  flex-direction: column;
  max-width: 600px;
  max-height: calc(75vh - ${({ theme }) => theme.space[11]}px);
`;

const StyledText = styled(Text)`
  white-space: pre-wrap;
  flex: 1 1 auto;
  overflow-y: auto;
`;
