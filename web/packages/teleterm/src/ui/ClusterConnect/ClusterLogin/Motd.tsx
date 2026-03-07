/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { Box, ButtonPrimary, Text } from 'design';

export function Motd({
  message,
  onAcknowledge,
  px,
}: {
  message: string;
  onAcknowledge(): void;
  px?: number | string;
}) {
  return (
    <Box px={px}>
      <StyledMessage typography="body1" mb={3}>
        {message}
      </StyledMessage>
      <ButtonPrimary block onClick={onAcknowledge}>
        Acknowledge
      </ButtonPrimary>
    </Box>
  );
}

const StyledMessage = styled(Text)`
  white-space: pre-wrap;
  overflow-y: auto;
  max-height: 300px;
`;
