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

import { formatDistanceToNowStrict } from 'date-fns';
import { PropsWithChildren } from 'react';

import Flex from 'design/Flex';
import { SyncAlt } from 'design/Icon';
import { P3 } from 'design/Text';

export function SyncStamp({
  date,
  children,
}: PropsWithChildren<{ date?: Date }>) {
  let content = 'N/A';
  if (date && date.getTime() >= 0) {
    content = formatDistanceToNowStrict(date, { addSuffix: true });
  }

  return (
    <Flex data-testid="sync" alignItems="center">
      <SyncAlt color="text.muted" size="small" mr={1} />
      <P3 color="text.muted">
        Last Sync: {content} {children}
      </P3>
    </Flex>
  );
}
