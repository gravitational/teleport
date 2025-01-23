/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Flex, P3 } from 'design';
import { Warning } from 'design/Icon';

export function ProfileStatusError(props: { error: string }) {
  return (
    <Flex>
      <Warning color="error.main" size="small" mr={1} />
      <P3
        color="text.slightlyMuted"
        css={`
          text-wrap: auto;
          line-height: 1.25;
        `}
      >
        {toWellFormattedConnectionError(props.error)}
      </P3>
    </Flex>
  );
}

/** Takes the last line from the error and capitalizes it. */
function toWellFormattedConnectionError(error: string): string {
  const lastLine = error.trim().split(/\r?\n/).at(-1).trim();
  return lastLine.charAt(0).toUpperCase() + lastLine.slice(1);
}
