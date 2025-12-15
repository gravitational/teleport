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

import { Flex, LabelInput, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';
import Select, { Option } from 'shared/components/Select';

export function AccessDurationRequest({
  maxDuration,
  onMaxDurationChange,
  maxDurationOptions,
}: {
  maxDurationOptions: Option<number>[];
  maxDuration: Option<number>;
  onMaxDurationChange(s: Option<number>): void;
}) {
  return (
    <LabelInput color="text.slightlyMuted">
      <Flex alignItems="center">
        <Text mr={1}>Access Duration</Text>
        <IconTooltip>
          How long you would be given elevated privileges. Note that the time it
          takes to approve this request will be subtracted from the duration you
          requested.
        </IconTooltip>
      </Flex>
      <Select
        options={maxDurationOptions}
        onChange={onMaxDurationChange}
        value={maxDuration}
      />
    </LabelInput>
  );
}
