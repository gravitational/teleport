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

import { useState, useEffect } from 'react';
import { Flex, LabelInput, Text } from 'design';

import Select, { Option } from 'shared/components/Select';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { AccessRequest } from 'shared/services/accessRequests';

import {
  getDurationOptionIndexClosestToOneWeek,
  getDurationOptionsFromStartTime,
} from './durationOptions';

export function AccessDurationRequest({
  assumeStartTime,
  accessRequest,
  maxDuration,
  setMaxDuration,
}: {
  assumeStartTime: Date;
  accessRequest: AccessRequest;
  maxDuration: Option<number>;
  setMaxDuration(s: Option<number>): void;
}) {
  // Options for extending or shortening the access request duration.
  const [durationOptions, setDurationOptions] = useState<Option<number>[]>([]);

  useEffect(() => {
    if (!assumeStartTime) {
      defaultDuration();
    } else {
      updateAccessDuration(assumeStartTime);
    }
  }, [assumeStartTime]);

  function defaultDuration() {
    const created = accessRequest.created;
    const options = getDurationOptionsFromStartTime(created, accessRequest);

    setDurationOptions(options);
    if (options.length > 0) {
      const durationIndex = getDurationOptionIndexClosestToOneWeek(
        options,
        accessRequest.created
      );
      setMaxDuration(options[durationIndex]);
    }
  }

  function updateAccessDuration(start: Date) {
    const updatedDurationOpts = getDurationOptionsFromStartTime(
      start,
      accessRequest
    );

    const durationIndex = getDurationOptionIndexClosestToOneWeek(
      updatedDurationOpts,
      start
    );

    setMaxDuration(updatedDurationOpts[durationIndex]);
    setDurationOptions(updatedDurationOpts);
  }

  return (
    <LabelInput typography="body2" color="text.slightlyMuted">
      <Flex alignItems="center">
        <Text mr={1}>Access Duration</Text>
        <ToolTipInfo>
          How long you would be given elevated privileges. Note that the time it
          takes to approve this request will be subtracted from the duration you
          requested.
        </ToolTipInfo>
      </Flex>

      <Select
        options={durationOptions}
        onChange={setMaxDuration}
        value={maxDuration}
      />
    </LabelInput>
  );
}
