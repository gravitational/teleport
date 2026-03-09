/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Box, ButtonPrimary, P2 } from 'design';

import { outermostPadding } from '../spacing';

export function MessageOfTheDay(props: {
  message: string;
  onAcknowledge(): void;
}) {
  return (
    <>
      {/* Make the internal container scrollable, so that the acknowledge button is always visible. */}
      <Box mb={3} maxHeight="400px" overflow="auto">
        <P2 whiteSpace="pre-wrap" px={outermostPadding}>
          {props.message}
        </P2>
      </Box>
      <ButtonPrimary
        size="large"
        mx={outermostPadding}
        autoFocus
        onClick={props.onAcknowledge}
      >
        Acknowledge
      </ButtonPrimary>
    </>
  );
}
