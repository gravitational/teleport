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

import { Box, Toggle } from 'design';
import { IconTooltip } from 'design/Tooltip';

export function AutoDiscoverToggle({
  wantAutoDiscover,
  toggleWantAutoDiscover,
  disabled = false,
}: {
  wantAutoDiscover: boolean;
  toggleWantAutoDiscover(): void;
  disabled?: boolean;
}) {
  return (
    <Box mb={2}>
      <Toggle
        isToggled={wantAutoDiscover}
        onToggle={toggleWantAutoDiscover}
        disabled={disabled}
      >
        <Box ml={2} mr={1}>
          Auto-enroll all databases for the selected VPC
        </Box>
        <IconTooltip>
          Auto-enroll will automatically identify all RDS databases (e.g.
          PostgreSQL, MySQL, Aurora) from the selected VPC and register them as
          database resources in your infrastructure.
        </IconTooltip>
      </Toggle>
    </Box>
  );
}
