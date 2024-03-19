/**
 * Copyright 2024 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState } from 'react';
import { Flex, Text, ButtonIcon, Box, LabelInput } from 'design';
import * as Icon from 'design/Icon';
import Select, { Option } from 'shared/components/Select';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { AccessRequest } from 'e-teleport/services/workflow';
import { getFormattedDurationTxt } from 'e-teleport/Workflow/Shared/utils';

export function AdditionalOptions({
  selectedMaxDurationTimestamp,
  requestTTLDurationOptions,
  setRequestTTL,
  requestTTL,
  dryRunResponse,
}: {
  selectedMaxDurationTimestamp: number;
  requestTTLDurationOptions: Option<number>[];
  setRequestTTL(o: Option<number>): void;
  requestTTL: Option<number>;
  dryRunResponse: AccessRequest;
}) {
  const [expanded, setExpanded] = useState(false);
  const ArrowIcon = expanded ? Icon.ChevronDown : Icon.ChevronRight;

  return (
    <>
      <Flex
        borderBottom={1}
        mt={1}
        mb={2}
        pb={2}
        justifyContent="space-between"
        alignItems="center"
        height="34px"
        css={`
          border-color: ${props => props.theme.colors.spotBackground[1]};
        `}
      >
        <Text mr={2} fontSize={1}>
          Additional Options
        </Text>
        <ButtonIcon
          onClick={() => setExpanded(e => !e)}
          data-testid="additional-info-btn"
        >
          <ArrowIcon size="medium" />
        </ButtonIcon>
      </Flex>
      {expanded && (
        <Box data-testid="reviewers">
          {requestTTLDurationOptions.length > 0 && (
            <LabelInput typography="body2" color="text.slightlyMuted" mb={3}>
              <Flex alignItems="center">
                <Text mr={1}>Request expires if not reviewed in</Text>
                <ToolTipInfo>
                  The amount of time this request will be in the PENDING state
                  before it expires.
                </ToolTipInfo>
              </Flex>
              <Select
                options={requestTTLDurationOptions}
                onChange={(option: Option<number>) => setRequestTTL(option)}
                value={requestTTL}
              />
            </LabelInput>
          )}
          <LabelInput typography="body2" color="text.slightlyMuted">
            <Flex alignItems="center">
              <Text mr={1}>Access Request Lifetime</Text>
              <ToolTipInfo>
                The total length of time the access request will exist for.
                Countdown starts from when an access request gets created to
                when the access request expires.
              </ToolTipInfo>
            </Flex>
            <Text>
              {getFormattedDurationTxt({
                start: dryRunResponse.created,
                end: new Date(selectedMaxDurationTimestamp),
              })}
            </Text>
          </LabelInput>
        </Box>
      )}
    </>
  );
}
