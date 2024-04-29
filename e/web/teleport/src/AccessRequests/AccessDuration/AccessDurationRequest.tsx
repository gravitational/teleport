import { useState, useEffect } from 'react';
import { Flex, LabelInput, Text } from 'design';
import Select, { Option } from 'shared/components/Select';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { AccessRequest } from 'e-teleport/services/accessRequests';

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
